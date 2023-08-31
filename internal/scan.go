package magellan

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)


func rawConnect(host string, ports []int, timeout int, keepOpenOnly bool) []bmcProbeResult {
	results := []bmcProbeResult{}
	for _, p := range ports {
		result := bmcProbeResult{
			Host:     host,
			Port:     p,
			Protocol: "tcp",
			State:    false,
		}
		t := time.Second * time.Duration(timeout)
		port := fmt.Sprint(p)
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), t)
		if err != nil {
			result.State = false
			// fmt.Println("Connecting error:", err)
		}
		if conn != nil {
			result.State = true
			defer conn.Close()
			// fmt.Println("Opened", net.JoinHostPort(host, port))
		}
		if keepOpenOnly {
			if result.State {
				results = append(results, result)
			}
		} else {
			results = append(results, result)
		}
	}

	return results
}

func GenerateHosts(subnet string, begin uint8, end uint8) []string {
	hosts := []string{}
	ip := net.ParseIP(subnet).To4()
	for i := begin; i < end; i++ {
		ip[3] = byte(i)
		hosts = append(hosts, fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3]))
	}
	return hosts
}

func ScanForAssets(hosts []string, ports []int, threads int, timeout int) []bmcProbeResult {
	states := make([]bmcProbeResult, 0, len(hosts))
	done := make(chan struct{}, threads+1)
	chanHost := make(chan string, threads+1)
	// chanPort := make(chan int, threads+1)
	var wg sync.WaitGroup
	wg.Add(threads)
	for i := 0; i < threads; i++ {
		go func() {
			for {
				host, ok := <-chanHost
				if !ok {
					wg.Done()
					return
				}
				s := rawConnect(host, ports, timeout, true)
				states = append(states, s...)
			}
		}()
	}

	for _, host := range hosts {
		chanHost <- host
	}
	go func() {
		select {
		case <-done:
			wg.Done()
			break
		default:
			time.Sleep(1000)
		}
	}()
	close(chanHost)
	wg.Wait()
	close(done)
	return states
}

func StoreStates(path string, states *[]bmcProbeResult) error {
	if states == nil {
		return fmt.Errorf("states == nil")
	}

	// create database if it doesn't already exist
	schema := `
	CREATE TABLE IF NOT EXISTS magellan_scanned_ports (
		host TEXT PRIMARY KEY NOT NULL,
		port INTEGER,
		protocol TEXT,
		state INTEGER
	);
	`
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("could not open database: %v", err)
	}
	db.MustExec(schema)

	// insert all probe states into db
	tx := db.MustBegin()
	for _, state := range *states {
		sql := `INSERT OR REPLACE INTO magellan_scanned_ports (host, port, protocol, state) 
		VALUES (:host, :port, :protocol, :state);`
		_, err := tx.NamedExec(sql, &state)
		if err != nil {
			fmt.Printf("could not execute transaction: %v\n", err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("could not commit transaction: %v", err)
	}
	return nil
}

func GetStates(path string) ([]bmcProbeResult, error) {
	db, err := sqlx.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %v", err)
	}

	results := []bmcProbeResult{}
	err = db.Select(&results, "SELECT * FROM magellan_scanned_ports ORDER BY host ASC")
	if err != nil {
		return nil, fmt.Errorf("could not retrieve probes: %v", err)
	}
	return results, nil
}

func GetDefaultPorts() []int {
	return []int{SSH_PORT, TLS_PORT, IPMI_PORT, REDFISH_PORT}
}