// Command openapi prints the API's OpenAPI 3.1 document to stdout.
package main

import (
	"fmt"
	"os"

	"bugun/internal/httpapi"
)

func main() {
	b, err := httpapi.Spec()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(b)
}
