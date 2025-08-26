package syncer

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateSHA256(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "SHA256 of non-empty file",
			content:  "hello world",
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9", // Хеш для "hello world"
		},
		{
			name:     "SHA256 of empty file",
			content:  "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // Хеш для пустого файла
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Создаем временный файл
			tmpFile, err := os.CreateTemp("", "testfile-*.txt")
			assert.NoError(t, err)          // Проверяем, что не произошло ошибки
			defer os.Remove(tmpFile.Name()) // Удаляем файл после теста

			// Записываем содержимое в файл
			_, err = tmpFile.Write([]byte(test.content))
			assert.NoError(t, err) // Проверяем, что не произошло ошибки
			tmpFile.Close()        // Закрываем файл, чтобы его можно было открыть для чтения

			// Вычисляем SHA256
			hash, err := CalculateSHA256(tmpFile.Name())
			assert.NoError(t, err) // Проверяем, что не произошло ошибки

			// Проверяем, что полученный хеш соответствует ожидаемому
			assert.Equal(t, test.expected, hash)
		})
	}

	// Тест для несуществующего файла
	t.Run("Non-existent file", func(t *testing.T) {
		_, err := CalculateSHA256("non_existent_file.txt")
		assert.Error(t, err) // Проверяем, что произошла ошибка
	})
}

func createTempDir(t *testing.T) (string, string) {
	srcDir, err := os.MkdirTemp("", "src")
	if err != nil {
		t.Fatal(err)
	}

	dstDir, err := os.MkdirTemp("", "dst")
	if err != nil {
		t.Fatal(err)
	}

	return srcDir, dstDir
}

// Тест для syncFile
func TestSyncFile(t *testing.T) {
	tests := []struct {
		name          string
		srcContent    string
		dstContent    string
		expectContent string
	}{
		{
			name:          "New file sync",
			srcContent:    "Hello, World!",
			dstContent:    "",
			expectContent: "Hello, World!",
		},
		{
			name:          "File already exists with different content",
			srcContent:    "New Content",
			dstContent:    "Old Content",
			expectContent: "New Content",
		},
		{
			name:          "File already exists with same content",
			srcContent:    "Same Content",
			dstContent:    "Same Content",
			expectContent: "Same Content",
		},
		{
			name:          "Empty source file",
			srcContent:    "",
			dstContent:    "Some Content",
			expectContent: "",
		},
		{
			name:          "Both files empty",
			srcContent:    "",
			dstContent:    "",
			expectContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcDir, dstDir := createTempDir(t)
			defer os.RemoveAll(srcDir)
			defer os.RemoveAll(dstDir)
			defer os.Remove("log.txt")

			// Создаем исходный файл
			os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte(tt.srcContent), os.ModePerm)

			// Создаем целевой файл
			os.WriteFile(filepath.Join(dstDir, "file.txt"), []byte(tt.dstContent), os.ModePerm)

			syncer := NewSyncer(srcDir, dstDir)

			// Запускаем синхронизацию
			err := syncer.syncFile(filepath.Join(srcDir, "file.txt"), filepath.Join(dstDir, "file.txt"))
			assert.NoError(t, err)

			// Проверяем, что содержимое целевого файла соответствует ожидаемому
			content, err := os.ReadFile(filepath.Join(dstDir, "file.txt"))
			assert.NoError(t, err)
			assert.Equal(t, tt.expectContent, string(content))
		})
	}
}

// Тест для syncDirectories
func TestSyncDirectories(t *testing.T) {
	tests := []struct {
		name        string
		srcFiles    []string
		dstFiles    []string
		expectFiles []string
	}{
		{
			name:        "Sync new files",
			srcFiles:    []string{"file1.txt", "file2.txt"},
			dstFiles:    []string{},
			expectFiles: []string{"file1.txt", "file2.txt"},
		},
		{
			name:        "Update existing file",
			srcFiles:    []string{"file.txt"},
			dstFiles:    []string{"file.txt"},
			expectFiles: []string{"file.txt"},
		},
		{
			name:        "Delete non-existing files",
			srcFiles:    []string{},
			dstFiles:    []string{"file1.txt", "file2.txt"},
			expectFiles: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcDir, dstDir := createTempDir(t)
			defer os.RemoveAll(srcDir)
			defer os.RemoveAll(dstDir)
			defer os.Remove("log.txt")

			// Создаем файлы в исходной директории
			for _, file := range tt.srcFiles {
				os.WriteFile(filepath.Join(srcDir, file), []byte("content"), os.ModePerm)
			}

			// Создаем файлы в целевой директории
			for _, file := range tt.dstFiles {
				os.WriteFile(filepath.Join(dstDir, file), []byte("content"), os.ModePerm)
			}

			syncer := NewSyncer(srcDir, dstDir)

			// Запускаем синхронизацию директорий
			syncer.syncDirectories()

			// Проверяем, что целевая директория содержит ожидаемые файлы
			for _, file := range tt.expectFiles {
				_, err := os.Stat(filepath.Join(dstDir, file))
				assert.NoError(t, err)
			}

			// Проверяем, что в целевой директории нет лишних файлов
			files, err := os.ReadDir(dstDir)
			assert.NoError(t, err)
			assert.Len(t, files, len(tt.expectFiles))
		})
	}
}
