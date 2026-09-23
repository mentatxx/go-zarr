package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/mentatxx/go-zarr"
	v2 "github.com/mentatxx/go-zarr/v2"
	v3 "github.com/mentatxx/go-zarr/v3"
)

func main() {
	arrayPath := flag.String("array_path", "", "Path to the Zarr array")
	flag.Parse()
	if *arrayPath == "" {
		fmt.Fprintln(os.Stderr, "--array_path is required")
		os.Exit(2)
	}
	ctx := context.Background()
	n, err := zarr.OpenPath(ctx, *arrayPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open array at %s\n%v\n", *arrayPath, err)
		os.Exit(1)
	}
	switch a := n.(type) {
	case *v3.Array:
		data, err := a.Read(ctx, nil, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read array: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(data.String())
	case *v2.Array:
		data, err := a.Read(ctx, nil, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read array: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(data.String())
	default:
		fmt.Fprintf(os.Stderr, "not an array: %T\n", n)
		os.Exit(1)
	}
}
