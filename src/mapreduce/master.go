package mapreduce

import "container/list"
import "fmt"

type WorkState int

const (
	NILWorkState WorkState = iota
	BUSY
	IDLE
)

type WorkerInfo struct {
	address string
	// You can add definitions here.  state WorkState
}

// Clean up all workers by sending a Shutdown RPC to each one of them Collect
// the number of jobs each work has performed.
func (mr *MapReduce) KillWorkers() *list.List {
	l := list.New()
	for _, w := range mr.Workers {
		DPrintf("DoWork: shutdown %s\n", w.address)
		args := &ShutdownArgs{}
		var reply ShutdownReply
		ok := call(w.address, "Worker.Shutdown", args, &reply)
		if ok == false {
			fmt.Printf("DoWork: RPC %s shutdown error\n", w.address)
		} else {
			l.PushBack(reply.Njobs)
		}
	}
	return l
}

func (mr *MapReduce) RunMaster() *list.List {

	jobArgsChan := make(chan DoJobArgs)

	go func() {
		// 源代码里面,这个registerChannel 没有关的函数,有点不太好.
		for worker := range mr.registerChannel {
			go func(worker string) {
				for args := range jobArgsChan {
					reply := &DoJobReply{}
					ok := call(worker, "Worker.DoJob", args, &reply)
					if !ok {
						fmt.Printf("DoWork: RPC %s do job error\n", worker)
					}
				}
			}(worker)
		}
	}()

	defer close(jobArgsChan)
	for i := 0; i < mr.nMap; i++ {
		args := DoJobArgs{
			mr.file, Map, i, mr.nReduce,
		}
		jobArgsChan <- args
	}
	for i := 0; i < mr.nReduce; i++ {
		args := DoJobArgs{
			mr.file, Reduce, i, mr.nMap,
		}
		jobArgsChan <- args
	}

	return mr.KillWorkers()
}
