package main

import (
	"sync"
	"sync/atomic"
	"testing"
)

func Test用户运行状态并发计数(t *testing.T) {
	state := 用户运行状态{
		用户设置:   make(map[string]user_info),
		用户ID映射: make(map[int]string),
		每小时请求数: make(map[string]int),
	}
	const workers = 500
	var accepted int64
	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer waitGroup.Done()
			if state.增加请求次数("tester", workers) {
				atomic.AddInt64(&accepted, 1)
			}
		}()
	}
	waitGroup.Wait()

	if accepted != workers || state.读取请求次数("tester") != workers {
		t.Fatalf("并发计数错误: accepted=%d count=%d", accepted, state.读取请求次数("tester"))
	}
	if state.增加请求次数("tester", workers) {
		t.Fatal("达到请求上限后不应继续接受请求")
	}
}

func Test用户运行状态索引一致(t *testing.T) {
	state := 用户运行状态{
		用户设置:   make(map[string]user_info),
		用户ID映射: make(map[int]string),
		每小时请求数: make(map[string]int),
	}
	state.保存用户设置(user_info{ID: 7, Name: "tester"})
	info, ok := state.读取用户设置("tester")
	if !ok || info.ID != 7 || state.读取用户名(7) != "tester" {
		t.Fatalf("用户设置与ID索引不一致: info=%+v ok=%v", info, ok)
	}
}
