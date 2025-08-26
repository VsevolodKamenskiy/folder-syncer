package syncer

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const SynchtoniztionPeriod = 10 * time.Second

type Syncer struct {
	srcDir string
	dstDir string
	mu     sync.Mutex
	logger *logrus.Logger
}

func NewSyncer(srcDir, dstDir string) *Syncer {
	logFile, err := os.OpenFile("log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("failed to open log file: %v\n", err)
	}
	logger := logrus.New()
	logger.SetOutput(logFile)
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	return &Syncer{
		srcDir: srcDir,
		dstDir: dstDir,
		logger: logger,
	}
}

func (s *Syncer) Sync(ctx context.Context) {
	ticker := time.NewTicker(SynchtoniztionPeriod) // Проверяем каждые n секунд
	s.syncDirectories()
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Infof("Synchronization stopped")
			return
		case <-ticker.C:
			s.syncDirectories()
		}
	}
}

func (s *Syncer) syncDirectories() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Infof("Syncing started")

	var wg sync.WaitGroup                 // Создаем WaitGroup
	toDelete := make(map[string]struct{}) // Карта для хранения путей, которые нужно удалить

	// Сначала синхронизируем файлы и папки из исходной директории
	err := filepath.WalkDir(s.srcDir, func(srcPath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(s.srcDir, srcPath)
		if err != nil {
			s.logger.Errorf("Error getting relative path for %s: %v", srcPath, err)
			return err
		}

		dstPath := filepath.Join(s.dstDir, relPath)

		perms, err := os.Stat(srcPath)
		if err != nil {
			s.logger.Errorf("Error reading permissions for %s: %v", srcPath, err)
			return err
		}

		if d.IsDir() {
			// Создаем директорию, если она не существует
			err := os.MkdirAll(dstPath, os.ModePerm)

			if err != nil {
				s.logger.Errorf("Error creating directory %s: %v", dstPath, err)
				return err
			}

			err = os.Chmod(dstPath, perms.Mode())
			if err != nil {
				s.logger.Errorf("Error changing permissions for directory %s: %v", dstPath, err)
				return err
			}

			return nil
		}

		// Запускаем синхронизацию файла как горутину
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.syncFile(srcPath, dstPath)
			if err != nil {
				s.logger.Errorf("Error syncing file: %v", err)
				return
			}
		}()

		return err
	})

	if err != nil {
		s.logger.Errorf("Error syncing directories: %v", err)
	}

	// Теперь проходим по целевой директории и отмечаем для удаления
	err = filepath.WalkDir(s.dstDir, func(dstPath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(s.dstDir, dstPath)

		if err != nil {
			s.logger.Errorf("Error getting relpath for dst file %s: %v", dstPath, err)
			return err
		}

		srcPath := filepath.Join(s.srcDir, relPath)

		// Если файл или директория не существуют в исходной директории, помечаем на удаление
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			toDelete[dstPath] = struct{}{}
		}

		return nil
	})

	if err != nil {
		s.logger.Errorf("Error checking destination directory: %v", err)
		return
	}

	// Удаляем помеченные файлы и директории
	for path := range toDelete {
		if err := os.RemoveAll(path); err != nil {
			s.logger.Errorf("Error deleting %s: %v", path, err)
		} else {
			s.logger.Infof("Deleted %s", path)
		}
	}

	wg.Wait() // Ждем завершения всех горутин
	s.logger.Infof("Synchronization done!")
}

func (s *Syncer) syncFile(srcPath, dstPath string) error {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	dstInfo, err := os.Stat(dstPath)
	if err == nil {
		if srcInfo.Size() == 0 && dstInfo.Size() == 0 {
			s.logger.Infof("Both files are empty. Skipping sync for: %s", dstPath)
			return nil
		}

		// Сравниваем хеши вместо размеров
		var wg sync.WaitGroup
		hashChan := make(chan string, 2)
		errChan := make(chan error, 2)

		wg.Add(2) // Добавляем две горутины

		go func() {
			defer wg.Done()
			hash, err := CalculateSHA256(srcPath)
			if err != nil {
				errChan <- err
			} else {
				hashChan <- hash
			}
		}()

		go func() {
			defer wg.Done()
			hash, err := CalculateSHA256(dstPath)
			if err != nil {
				errChan <- err
			} else {
				hashChan <- hash
			}
		}()

		go func() {
			wg.Wait()
			close(hashChan)
			close(errChan)
		}()

		var srcHash, dstHash string
		for h := range hashChan {
			if srcHash == "" {
				srcHash = h
			} else {
				dstHash = h
			}
		}

		// Проверяем на наличие ошибок
		for err := range errChan {
			return err
		}

		if srcHash == dstHash {
			s.logger.Infof("File %s already exists with the same SHA-256 hash, skipping.", dstPath)
			return nil
		}
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}

	if err := os.Chmod(dstPath, srcInfo.Mode()); err != nil {
		return err
	}

	defer dstFile.Close()

	n, err := io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	s.logger.Infof("Synced file: %s to %s, size: %d bytes", srcPath, dstPath, n)
	return nil
}
