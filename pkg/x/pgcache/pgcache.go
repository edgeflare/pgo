package pgcache

import (
	"fmt"
	"io"
	"log"
	"net"

	pg_query "github.com/pganalyze/pg_query_go/v5"
)

func main() {
	listener, err := net.Listen("tcp", "localhost:5430")
	if err != nil {
		log.Fatalf("Error starting TCP server: %v", err)
	}
	defer listener.Close()
	log.Println("Server is listening on localhost:5430")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		go handleClient(conn)
	}
}

func handleClient(clientConn net.Conn) {
	defer clientConn.Close()

	serverConn, err := net.Dial("tcp", "localhost:5443")
	if err != nil {
		log.Printf("Error connecting to PostgreSQL server: %v", err)
		return
	}
	defer serverConn.Close()

	go func() {
		// Forward responses from PostgreSQL server to the client
		if _, err := io.Copy(clientConn, serverConn); err != nil {
			log.Printf("Error forwarding response to client: %v", err)
		}
	}()

	// Read and parse client queries
	buf := make([]byte, 4096)
	for {
		n, err := clientConn.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from client: %v", err)
			}
			break
		}

		query := string(buf[:n])
		log.Printf("Received data: %s", query)

		// Skip non-SQL messages (like startup messages)
		// if !isSQLQuery(query) {
		// 	log.Printf("Skipping non-SQL message: %s", query)
		// 	// Forward the message to the PostgreSQL server
		// 	if _, err := serverConn.Write(buf[:n]); err != nil {
		// 		log.Printf("Error forwarding message to PostgreSQL server: %v", err)
		// 		break
		// 	}
		// 	continue
		// }

		// Parse and log metrics
		parseQuery(query)

		// Forward the query to the PostgreSQL server
		if _, err := serverConn.Write(buf[:n]); err != nil {
			log.Printf("Error forwarding query to PostgreSQL server: %v", err)
			break
		}
	}
}

// func isSQLQuery(query string) bool {
// 	// Simple heuristic to check if the message is a SQL query
// 	query = strings.TrimSpace(query)
// 	if len(query) == 0 {
// 		return false
// 	}

// 	firstChar := query[0]
// 	// Check if the first character is likely the start of a SQL command
// 	return firstChar == 'S' || firstChar == 'I' || firstChar == 'U' || firstChar == 'D' || firstChar == 's' || firstChar == 'i' || firstChar == 'u' || firstChar == 'd'
// }

func parseQuery(query string) {
	tree, err := pg_query.ParseToJSON(query)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("\n%+v\n", tree)
}
