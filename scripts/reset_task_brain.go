package main
import (
	"log"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
)
func main() {
	store, _ := storage.NewStorage("swarm.db")
	store.DB.Model(&storage.SwarmTask{}).Where("id = 1").Update("status", storage.StatusPending)
	log.Println("Task #1 reset to PENDING for Brain-Detection test.")
}
