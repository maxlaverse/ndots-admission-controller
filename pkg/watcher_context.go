package pkg

import (
	"context"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"k8s.io/klog/v2"
)

func directoryWatcherContext(ctx context.Context, directory string) (context.Context, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	err = watcher.Add(directory)
	if err != nil {
		return nil, err
	}

	newCtx, cancel := context.WithCancel(ctx)
	go func() {
		klog.V(1).Infof("Watching directory '%v'", watcher.WatchList())
		defer func() {
			klog.V(0).Infof("Files in watched directory have been modified, cancelling context")
			cancel()
			watcher.Close()
		}()

		for {
			select {
			case event, ok := <-watcher.Events:
				if ok {
					klog.V(1).Infof("event '%s' on '%s'", event.Op.String(), event.Name)

					/*
						When a mounted Secret is updated, the following events can be observed:
						* CREATE ..2024_01_08_11_42_04.2575209046
						* CHMOD ..2024_01_08_11_42_04.2575209046
						* CREATE ..data_tmp
						* RENAME ..data_tmp
						* CREATE ..data
						* REMOVE	..2024_01_08_11_40_42.3496535436

						We want to cancell the context when most of the work has been done,
						which is when the '..data' file is created (renamed to).
					*/
					if event.Has(fsnotify.Create) && strings.HasSuffix(event.Name, "/..data") {
						time.Sleep(watcherShutdownDelay)
						return
					}
				}
			case err := <-watcher.Errors:
				klog.Errorf("watcher error: %v", err)
			case <-ctx.Done():
			}
		}
	}()

	return newCtx, nil
}
