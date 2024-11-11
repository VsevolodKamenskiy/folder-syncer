package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syncer/internal/syncer"
	"syscall"
)

func main() {
	if len(os.Args) > 3 {
		fmt.Println("Usage: go run cmd/app/main.go <source_directory> <destination_directory>")
		os.Exit(127)
	}

	srcDir := os.Args[1] // путь к исходной директории
	dstDir := os.Args[2] // путь к директории назначения

	// Проверяем существование исходной директории
	if _, err := os.Stat(srcDir); err != nil {
		fmt.Println("Source directory error:", err)
		os.Exit(127)
	}

	// Директория назначения не проверяется, т.к если ее нет, то она просто создастся в папке с проектом (баг или фича?)

	folderSyncer := syncer.NewSyncer(srcDir, dstDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go folderSyncer.Sync(ctx) // Запускаем синхронизацию в горутине

	// Обработка сигналов для завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan // Ждем сигнала завершения
}
