package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boot.dev/linko/internal/store"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	httpPort := flag.Int("port", 8899, "port to listen on")
	dataDir := flag.String("data", "./data", "directory to store data")
	flag.Parse()

	status := run(ctx, cancel, *httpPort, *dataDir)
	cancel()
	os.Exit(status)
}

func initLogger() (*log.Logger, func() error, error) {
	fName, isLogFile := os.LookupEnv("LINKO_LOG_FILE")
	if !isLogFile {
		return log.New(os.Stderr, "", log.LstdFlags), func() error { return nil }, nil
	}

	logFile, err := os.OpenFile(fName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, func() error {
			return nil
		}, err
	}

	bufFile := bufio.NewWriterSize(logFile, 1024)

	mWriter := io.MultiWriter(os.Stderr, bufFile)
	return log.New(mWriter, "", log.LstdFlags), func() error {

		if err := bufFile.Flush(); err != nil {
			return err
		}
		return logFile.Close()

	}, nil
}

func run(ctx context.Context, cancel context.CancelFunc, httpPort int, dataDir string) int {
	logger, closeLogger, err := initLogger()

	defer func() {
		if err := closeLogger(); err != nil {
			fmt.Fprintf(os.Stderr, "error when closing logger %w", err)
		}
	}()

	if err != nil {
		fmt.Errorf("err")
		return 1
	}

	st, err := store.New(dataDir, logger)
	if err != nil {
		logger.Printf("failed to create store: %v", err)
		return 1
	}
	s := newServer(*st, httpPort, cancel, logger)
	var serverErr error
	go func() {
		serverErr = s.start()
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.shutdown(shutdownCtx); err != nil {
		logger.Printf("failed to shutdown server: %v", err)
		return 1
	}
	if serverErr != nil {
		logger.Printf("server error: %v", serverErr)
		return 1
	}
	return 0
}
