package grpctransport_test

import (
	"fmt"

	"github.com/opendecree/decree/sdk/grpctransport"
)

func ExampleDial() {
	// TLS with system roots — production default.
	conn, err := grpctransport.Dial("passthrough:///localhost:50051")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer conn.Close()
	fmt.Println("ok")
	// Output: ok
}

func ExampleDial_insecure() {
	// Insecure — local development and testing only.
	conn, err := grpctransport.Dial("passthrough:///localhost:50051", grpctransport.WithInsecure())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer conn.Close()
	fmt.Println("ok")
	// Output: ok
}

func ExampleNewConfigClient() {
	conn, err := grpctransport.Dial("passthrough:///localhost:50051", grpctransport.WithInsecure())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer conn.Close()

	_, err = grpctransport.NewConfigClient(conn,
		grpctransport.WithSubject("myapp"),
		grpctransport.WithRole("user"),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("ok")
	// Output: ok
}

func ExampleNewAdminClient() {
	conn, err := grpctransport.Dial("passthrough:///localhost:50051", grpctransport.WithInsecure())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer conn.Close()

	_, err = grpctransport.NewAdminClient(conn,
		grpctransport.WithSubject("admin"),
		grpctransport.WithRole("superadmin"),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("ok")
	// Output: ok
}

func ExampleNewWatcher() {
	conn, err := grpctransport.Dial("passthrough:///localhost:50051", grpctransport.WithInsecure())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer conn.Close()

	_, err = grpctransport.NewWatcher(conn, "tenant-uuid",
		grpctransport.WithSubject("myapp"),
		grpctransport.WithRole("user"),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("ok")
	// Output: ok
}
