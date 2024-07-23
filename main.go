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
)

func main() {
    client := &http.Client{}
    group, ctx := errgroup.WithContext(context.Background())
    nextWriter := newDribbleWriter(ctx, os.Stdout)
    scanner := bufio.NewScanner(os.Stdin)
    i := 0
Scan:
    for scanner.Scan() {
        select {
        case <-ctx.Done():
            break Scan
        default:
        }
        urlStr := strings.TrimSpace(scanner.Text())
        if urlStr == "" {
            break
        }
        tmpI := i
        writer := nextWriter
        nextWriter = writer.next()
        group.Go(func() error {
            resp, err := client.Get(urlStr)
            if err != nil {
                return err
            }
            defer resp.Body.Close()

            if resp.StatusCode != 200 {
                return fmt.Errorf("expected 200 status code, got %v", resp.Status)
            }
            n, err := io.Copy(writer, resp.Body)
            log.Printf("finished with url %d (%d bytes)", tmpI, n)
            return err
        })
        i++
    }
    err := scanner.Err()
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
    err = group.Wait()
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
