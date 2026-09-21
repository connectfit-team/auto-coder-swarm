package main

import (
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// 딸린 것이 안 떴으면 작업을 집지 않는다.
//
// 이 기계는 하루 세 번(05·13·21시) 다시 뜬다. 그때 작업은 되살아나는데
// 딸린 서비스는 아직 뜨는 중이다. 실측으로 21:02:15 에 되살아난 작업이
// 이렇게 돌았다(W-43067).
//
//	[KNOWLEDGE_MISSING] 사내지식 없이 진행한다 —
//	  dial tcp 127.0.0.1:8007: connection refused
//
// 지식 없이 도는 것은 조용한 품질 저하다. 몇 초 기다리면 되는 일에
// 그것을 감수할 이유가 없다 — 사용자의 기준도 「느려도 괜찮고 확실한 작동만」이다.
//
// 못 뜬 것을 실패로 만들지도 않는다. 집지 않고 큐에 두면 뜬 뒤에 돈다.

type dep struct {
	name string
	addr string
}

func depsFromEnv() []dep {
	return []dep{
		{"사내지식(CKH)", hostPort(os.Getenv("CKH_URL"), "127.0.0.1:8007")},
		{"코드 분석(CIE)", hostPort(os.Getenv("CIE_URL"), "127.0.0.1:8005")},
	}
}

// hostPort 는 주소에서 host:port 만 뽑는다.
func hostPort(url, def string) string {
	if url == "" {
		return def
	}
	s := url
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?"); i >= 0 {
		s = s[:i]
	}
	if !strings.Contains(s, ":") {
		return def
	}
	return s
}

var (
	depMu     sync.Mutex
	depWaited bool
)

// depsReady 는 딸린 것이 다 떴는지 본다.
// 로그는 상태가 바뀔 때만 남긴다 — 2초마다 같은 줄을 찍으면 아무도 안 읽는다.
func depsReady() bool {
	var down []string
	for _, d := range depsFromEnv() {
		c, err := net.DialTimeout("tcp", d.addr, 2*time.Second)
		if err != nil {
			down = append(down, d.name)
			continue
		}
		c.Close()
	}

	depMu.Lock()
	defer depMu.Unlock()
	if len(down) == 0 {
		if depWaited {
			log.Printf("🟢 딸린 것이 다 떴다. 밀린 작업을 집는다")
			depWaited = false
		}
		return true
	}
	if !depWaited {
		log.Printf("⏸ %s 이 아직 안 떴다 — 작업을 큐에 둔다", strings.Join(down, "·"))
		depWaited = true
	}
	return false
}
