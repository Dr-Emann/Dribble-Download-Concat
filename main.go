package main

import (
	"bufio"
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func dribbleDownload(ctx context.Context, client *http.Client, url string, writeTo <-chan io.Writer, nextWriter chan<- io.Writer) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("expected 200 status code, got %v", resp.Status)
	}

	var readData []byte
	chunk := [4 * 1024]byte{}

	var writer io.Writer
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
OuterLoop:
	for {
		select {
		case writer = <-writeTo:
			break OuterLoop
		case _ = <-ticker.C:
		case _ = <-ctx.Done():
			return ctx.Err()
		}
		// Only get here when we're still slow reading
		n, err := resp.Body.Read(chunk[:])
		if err != nil {
			return err
		}
		readData = append(readData, chunk[:n]...)
	}
	// We now have the writer
	_, err = writer.Write(readData)
	if err != nil {
		return fmt.Errorf("error writing chunk: %w", err)
	}

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return fmt.Errorf("error copying body to writer: %w", err)
	}
	if nextWriter != nil {
		nextWriter <- writer
	}
	return nil
}

func main() {
	client := &http.Client{}
	group, ctx := errgroup.WithContext(context.Background())
	writer := os.Stdout
	lastWriterChan := make(chan io.Writer, 1)
	lastWriterChan <- writer
	scanner := bufio.NewScanner(os.Stdin)
	i := 0
	for scanner.Scan() {
		urlStr := strings.TrimSpace(scanner.Text())
		if urlStr == "" {
			break
		}
		inWriterChan := lastWriterChan
		nextWriterChan := make(chan io.Writer, 1)
		tmpI := i
		group.Go(func() error {
			err := dribbleDownload(ctx, client, urlStr, inWriterChan, nextWriterChan)
			log.Printf("finished with url %d", tmpI)
			return err
		})
		lastWriterChan = nextWriterChan
		i++
	}
	err := group.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
