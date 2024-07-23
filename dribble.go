package main

import (
    "bytes"
    "context"
    "io"
    "time"
)

type dribbleWriter struct {
    ctx           context.Context
    writer        io.Writer
    writeReceiver <-chan io.Writer
    nextWriter    chan io.Writer
    buf           bytes.Buffer
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

func (w *dribbleWriter) tryGetWriterOrWait() (io.Writer, error) {
    if w.writer != nil {
        return w.writer, nil
    }
    timer := time.NewTimer(5 * time.Second)
    defer timer.Stop()

    select {
    case w.writer = <-w.writeReceiver:
    case _ = <-timer.C:
    case _ = <-w.ctx.Done():
        return nil, w.ctx.Err()
    }
    if w.writer != nil {
        _, err := io.Copy(w.writer, &w.buf)
        w.buf = bytes.Buffer{}
        if err != nil {
            return nil, err
        }
    }
    return w.writer, nil
}

func (w *dribbleWriter) Write(p []byte) (int, error) {
    writer, err := w.tryGetWriterOrWait()
    if err != nil {
        return 0, err
    }
    if writer != nil {
        return w.writer.Write(p)
    }
    return w.buf.Write(p)
}

func (w *dribbleWriter) ReadFrom(r io.Reader) (n int64, err error) {
    for {
        writer, err := w.tryGetWriterOrWait()
        if err != nil {
            return n, err
        }
        if writer != nil {
            rest, err := io.Copy(writer, r)
            return n + rest, err
        }
        readN, err := io.CopyN(&w.buf, r, 4*1024)
        n += readN
    }
}
