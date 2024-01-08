package pkg

import (
	"context"
	"time"

	"github.com/fsnotify/fsnotify"
	"k8s.io/klog/v2"
)

const (
	shutdownDelay = 1 * time.Second
)

func fileWatcherContext(ctx context.Context, filepaths ...string) (context.Context, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, filepath := range filepaths {
		err = watcher.Add(filepath)
		if err != nil {
			return nil, err
		}
	}

	newCtx, cancel := context.WithCancel(ctx)
	go func() {
		klog.V(1).Infof("Watching files: %v", watcher.WatchList())
		defer func() {
			klog.V(0).Infof("Watched files have been modified, cancelling context")
			cancel()
			watcher.Close()
		}()

		for {
			select {
			case event, ok := <-watcher.Events:
				if ok {
					klog.V(1).Infof("event '%s' on '%s'", event.Op.String(), event.Name)

					// Act on write events only, as we're unsure if Kubelet might perform noop
					// chmod from time to time, e.g. when the Secrets are periodically synced.
					if !event.Has(fsnotify.Write) {
						break
					}

					// Artificially delays cancelling the context to avoid inconsistencies if
					// all the files being watched haven't be written yet. We could keep track of
					// their modification, and even check if they represent a consistent key pair,
					// but it's probably not worth it at this point.
					time.Sleep(shutdownDelay)
					return
				}
			case err := <-watcher.Errors:
				klog.Errorf("watcher error:", err)
			case <-ctx.Done():
			}
		}
	}()

	return newCtx, nil
}
