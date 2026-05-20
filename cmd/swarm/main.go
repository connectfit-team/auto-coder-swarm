package main

import (
	"context"
	"log"
	"time"

	"github.com/connectfit-team/auto-coder-swarm/internal/insightclient"
	"github.com/connectfit-team/auto-coder-swarm/internal/llm"
	"github.com/connectfit-team/auto-coder-swarm/internal/orchestrator"
)

func main() {
	log.Println("🚀 Auto-Coder Swarm Starting (Phase 2: LLM Brain Connected)")

	// 1. Initialize Dependencies
	ollamaModel := llm.NewOllamaModel("gemma4:31b", "http://localhost:11434")
	ic := insightclient.NewClient("http://localhost:8005")
	orc := orchestrator.NewSwarmOrchestrator(ic, ollamaModel)

	// 2. Define a real task
	userRequest := "gig_mobile 레포지토리의 easy_locale_manager.dart 파일에서 deviceLanguage 함수가 한국어(KR)를 명시적으로 처리하지 않고 있다면, 한국어를 인식하도록 코드를 수정해줘."

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	// 3. Run Pipeline
	if err := orc.RunTask(ctx, userRequest); err != nil {
		log.Fatalf("❌ Swarm task failed: %v", err)
	}

	log.Println("✅ Swarm task pipeline finished successfully.")
}
