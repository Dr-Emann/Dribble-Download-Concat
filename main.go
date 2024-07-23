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

type dribbleWriter struct {
    ctx           context.Context
    writer        io.Writer
    writeReceiver <-chan io.Writer
    nextWriter    chan io.Writer
    buf           []byte
}

func newDribbleWriter(ctx context.Context, writer io.Writer) *dribbleWriter {
    nextWriter := make(chan io.Writer, 1)
    return &dribbleWriter{
        ctx:        ctx,
        writer:     writer,
        nextWriter: nextWriter,
    }
}

func (w *dribbleWriter) next() *dribbleWriter {
    return &dribbleWriter{
        ctx:           w.ctx,
        writeReceiver: w.nextWriter,
        nextWriter:    make(chan io.Writer, 1),
    }
}

func (w *dribbleWriter) Close() error {
    if w.writer != nil {
        if closer, ok := w.writer.(io.Closer); ok {
            err := closer.Close()
            if err != nil {
                return err
            }
        }
        w.nextWriter <- w.writer
    }
    return nil
}

func (w *dribbleWriter) Write(p []byte) (int, error) {
    if w.writer != nil {
        if w.ctx.Err() != nil {
            return 0, w.ctx.Err()
        }
        return w.writer.Write(p)
    }
    timer := time.NewTimer(5 * time.Second)
    defer timer.Stop()

    select {
    case w.writer = <-w.writeReceiver:
    case _ = <-timer.C:
    case _ = <-w.ctx.Done():
        return 0, w.ctx.Err()
    }
    if w.writer != nil {
        _, err := w.writer.Write(w.buf)
        if err != nil {
            return 0, err
        }
        return w.writer.Write(p)
    }
    w.buf = append(w.buf, p...)
    return len(p), nil
}

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
