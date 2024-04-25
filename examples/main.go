package main

import (
	"context"
	"fmt"
	"time"

	"github.com/0xsequence/runnable"
)

type Monitor struct {
	runnable.Runnable
}

func NewMonitor() *Monitor {
	m := &Monitor{}
	m.Runnable = runnable.New(m.run)
	return m
}

func (m *Monitor) run(ctx context.Context) error {
	fmt.Println("Starting...")
	defer fmt.Println("Stopping...")

	// Start monitoring
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		time.Sleep(1 * time.Second)
		fmt.Println("Monitoring...")
	}
	return nil
}

func main() {
	// simple function
	fmt.Println("Simple function...")
	err := runnable.New(func(ctx context.Context) error {
		fmt.Println("Starting...")
		defer fmt.Println("Stopping...")

		for i := 0; i < 5; i++ {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			time.Sleep(1 * time.Second)
			fmt.Println("Running...")
		}
		return nil
	}).Run(context.Background())
	if err != nil {
		fmt.Println(err)
	}

	// simple function with stop
	fmt.Println("Simple function with stop...")
	r := runnable.New(func(ctx context.Context) error {
		fmt.Println("Starting...")
		defer fmt.Println("Stopping...")

		for {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			time.Sleep(1 * time.Second)
			fmt.Println("Running...")
		}
	})

	go func() {
		time.Sleep(5 * time.Second)

		fmt.Println("Calling Stop...")
		err := r.Stop(context.Background())
		if err != nil {
			fmt.Println(err)
		}
	}()

	err = r.Run(context.Background())
	if err != nil {
		fmt.Println(err)
	}

	// simple function with timeout
	fmt.Println("Simple function with timeout...")
	ctxWithTimeout, _ := context.WithTimeout(context.Background(), 5*time.Second)
	err = runnable.New(func(ctx context.Context) error {
		fmt.Println("Starting...")
		defer fmt.Println("Stopping...")

		for {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			time.Sleep(1 * time.Second)
			fmt.Println("Running...")
		}
	}).Run(ctxWithTimeout)
	if err != nil {
		fmt.Println(err)
	}

	// simple function with retry
	fmt.Println("Simple function with retry...")
	errorReturned := false
	err = runnable.New(func(ctx context.Context) error {
		fmt.Println("Starting...")
		defer fmt.Println("Stopping...")

		if !errorReturned {
			errorReturned = true
			return fmt.Errorf("error")
		}

		// do something
		for i := 0; i < 5; i++ {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			time.Sleep(1 * time.Second)
			fmt.Println("Running...")
		}
		return nil
	}, runnable.WithRetry(3, runnable.ResetNever)).Run(context.Background())
	if err != nil {
		fmt.Println(err)
	}

	// Runnable object
	fmt.Println("Runnable object(Monitor)...")
	m := NewMonitor()

	go func() {
		time.Sleep(5 * time.Second)

		fmt.Println("Calling Stop...")
		err := m.Stop(context.Background())
		if err != nil {
			fmt.Println(err)
		}
	}()

	err = m.Run(context.Background())
	if err != nil {
		fmt.Println(err)
	}
}
