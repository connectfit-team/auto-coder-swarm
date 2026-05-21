package main
import (
	"fmt"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
)
func main() {
	store, _ := storage.NewStorage("swarm.db")
	logs, _ := store.GetLogs(4)
	for _, l := range logs {
		fmt.Printf("[%s] %s: %s\n", l.CreatedAt, l.Stage, l.Message)
	}
}
