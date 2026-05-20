package voter

import (
	"context"
	"github.com/connectfit-team/auto-coder-swarm/internal/agent"
)

type SwarmVoter struct {
	MultiModelVoter
}

func (v *MultiModelVoter) ConsensusProcess(ctx context.Context, agent agent.Agent, input string) (string, error) {
	res, err := v.Vote(ctx, agent.Name(), "[PROMPT FOR CONSENSUS]\n"+input)
	if err != nil { return "", err }
	return res.Winner, nil
}
