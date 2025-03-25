package main

// This example demonstrates the usage of the LocalSecretStore to store and retrieve secrets.
// It provides a command-line interface to generate a master key, store secrets, and retrieve them.
// The master key is assumed to be stored in the environment variable MASTER_KEY and while it can
// anything you want, we recommend a 32 bit key for AES-256 encryption.  The master key is used
// as part of a Key Derivation Function (KDF) to generate a unique AES key for each secret.
// The algorithm of choice is HMAC-based Extract-and-Expand Key Derivation Function (HKDF).
// Each secret is separately encrypted using AES-GCM and stored in a JSON file.
// The JSON file is loaded into memory when the LocalSecretStore is created and saved back to the file
// when a secret is stored or removed.
//

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/OpenCHAMI/magellan/pkg/secrets"
)

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  go run main.go generatekey")
	fmt.Println("    - Generates a new 32-byte master key (in hex).")
	fmt.Println()
	fmt.Println("  Export MASTER_KEY=<your master key> to use the same key in the next commands.")
	fmt.Println()
	fmt.Println("  go run main.go store <secretID> <secretValue> [filename]")
	fmt.Println("    - Stores the given string value under secretID.")
	fmt.Println()
	fmt.Println("  go run main.go storebase64 <secretID> <base64String> [filename]")
	fmt.Println("    - Decodes the base64-encoded string before storing.")
	fmt.Println()
	fmt.Println("  go run main.go storejson <secretID> <jsonString> [filename]")
	fmt.Println("    - Stores the provided JSON for the specified secretID.")
	fmt.Println()
	fmt.Println("  go run main.go retrieve <secretID> [filename]")
	fmt.Println("    - Retrieves and prints the secret value for the given secretID.")
	fmt.Println()
	fmt.Println("  go run main.go list [filename]")
	fmt.Println("    - Lists all the secret IDs and their values.")
	fmt.Println()
}

// openStore tries to create or open the LocalSecretStore based on the environment
// variable MASTER_KEY. If not found, it prints an error.
func openStore(filename string) (*secrets.LocalSecretStore, error) {
	masterKey := os.Getenv("MASTER_KEY")
	if masterKey == "" {
		return nil, fmt.Errorf("MASTER_KEY environment variable not set")
	}

	store, err := secrets.NewLocalSecretStore(masterKey, filename, true)
	if err != nil {
		return nil, fmt.Errorf("cannot open secrets store: %v", err)
	}
	return store, nil
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "generatekey":
		key, err := secrets.GenerateMasterKey()
		if err != nil {
			fmt.Printf("Error generating master key: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", key)

	case "store":
		if len(os.Args) < 4 {
			fmt.Println("Not enough arguments. Usage: go run main.go store <secretID> <secretValue> [filename]")
			os.Exit(1)
		}
		secretID := os.Args[2]
		secretValue := os.Args[3]
		filename := "mysecrets.json"
		if len(os.Args) == 5 {
			filename = os.Args[4]
		}

		store, err := openStore(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := store.StoreSecretByID(secretID, secretValue); err != nil {
			fmt.Printf("Error storing secret: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Secret stored successfully.")

	case "storebase64":
		if len(os.Args) < 4 {
			fmt.Println("Not enough arguments. Usage: go run main.go storebase64 <secretID> <base64String> [filename]")
			os.Exit(1)
		}
		secretID := os.Args[2]
		base64Value := os.Args[3]
		filename := "mysecrets.json"
		if len(os.Args) == 5 {
			filename = os.Args[4]
		}

		decoded, err := base64.StdEncoding.DecodeString(base64Value)
		if err != nil {
			fmt.Printf("Error decoding base64 data: %v\n", err)
			os.Exit(1)
		}

		store, err := openStore(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := store.StoreSecretByID(secretID, string(decoded)); err != nil {
			fmt.Printf("Error storing base64-decoded secret: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Base64-decoded secret stored successfully.")

	case "storejson":
		if len(os.Args) < 4 {
			fmt.Println(`Not enough arguments. Usage: go run main.go storejson <secretID> '{"key":"value"}' [filename]`)
			os.Exit(1)
		}
		secretID := os.Args[2]
		jsonValue := os.Args[3]
		filename := "mysecrets.json"
		if len(os.Args) == 5 {
			filename = os.Args[4]
		}

		store, err := openStore(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if err := store.StoreSecretByID(secretID, jsonValue); err != nil {
			fmt.Printf("Error storing JSON secret: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("JSON secret stored successfully.")

	case "retrieve":
		if len(os.Args) < 3 {
			fmt.Println("Not enough arguments. Usage: go run main.go retrieve <secretID> [filename]")
			os.Exit(1)
		}
		secretID := os.Args[2]
		filename := "mysecrets.json"
		if len(os.Args) == 4 {
			filename = os.Args[3]
		}

		store, err := openStore(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		secretValue, err := store.GetSecretByID(secretID)
		if err != nil {
			fmt.Printf("Error retrieving secret: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Secret for %s: %s\n", secretID, secretValue)

	case "list":
		if len(os.Args) < 2 {
			fmt.Println("Not enough arguments. Usage: go run main.go list [filename]")
			os.Exit(1)
		}

		filename := "mysecrets.json"
		if len(os.Args) == 3 {
			filename = os.Args[2]
		}

		store, err := openStore(filename)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		secrets, err := store.ListSecrets()
		if err != nil {
			fmt.Printf("Error listing secrets: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Secrets:")
		for key, value := range secrets {
			fmt.Printf("%s: %s\n", key, value)
		}

	default:
		usage()
	}

}
