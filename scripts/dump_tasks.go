package main
import (
	"fmt"
	"github.com/connectfit-team/auto-coder-swarm/internal/storage"
)
func main() {
	store, _ := storage.NewStorage("swarm.db")
	tasks, _ := store.GetAllTasks()
	for _, t := range tasks {
		fmt.Printf("ID: %d, Repo: %s, Status: %s, Updated: %s\n", t.ID, t.RepoName, t.Status, t.UpdatedAt)
	}
}
