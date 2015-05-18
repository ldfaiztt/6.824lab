package viewservice

import "net"
import "net/rpc"
import "log"
import "time"
import "sync"
import "fmt"
import "os"
import "sync/atomic"

type State int

const (
	DEAD State = iota
	BUSY
	IDLE
)

type ViewServer struct {
	mu       sync.Mutex
	l        net.Listener
	dead     int32 // for testing
	rpccount int32 // for testing
	me       string

	// Your declarations here.
	clients      map[string]State // list of clients states.
	deadCount    map[string]int64 // dead interval count for a client.
	lastViewNum  map[string]uint  // last view number for client.
	currentView  View             // current view.
	previousView View             // previous view.
	acked        bool             // wether current view is accepted by a server.
}

func (vs *ViewServer) findIdleServer() string {
	for name, state := range vs.clients {
		if state == IDLE {
			log.Println("find", name, " marked BUSY.")
			vs.clients[name] = BUSY
			return name
		}
	}
	return ""
}

var once = true

func (vs *ViewServer) RerangePB(primary, backup string) {
	var view View
	if vs.currentView.Primary == "" && vs.currentView.Backup == "" {
		var change bool
		s := vs.findIdleServer()
		if s != "" {
			view.Primary = s
			change = true
		}
		s = vs.findIdleServer()
		if s != "" {
			view.Backup = s
			change = true
		}
		if change {
			view.Viewnum = vs.currentView.Viewnum + 1
			if primary != "" {
				vs.currentView.Primary = primary
			}
			if backup != "" {
				vs.currentView.Backup = backup
			}
			vs.previousView = vs.currentView
			vs.currentView = view
			if !once {
				vs.acked = false
			}
			once = false
		}
	}

	if vs.currentView.Primary == "" && vs.currentView.Backup != "" {
		s := vs.findIdleServer()
		if s != "" {

			view.Primary = vs.currentView.Backup
			view.Backup = s
			view.Viewnum = vs.currentView.Viewnum + 1

			if primary != "" {
				vs.currentView.Primary = primary
			}

			vs.previousView = vs.currentView
			vs.currentView = view
		} else {
			view.Primary = vs.currentView.Backup
			view.Viewnum = vs.currentView.Viewnum + 1

			if primary != "" {
				vs.currentView.Primary = primary
			}

			vs.previousView = vs.currentView
			vs.currentView = view
		}
		if !once {
			vs.acked = false
		}
		once = false
	}

	if vs.currentView.Primary != "" && vs.currentView.Backup == "" {
		view.Primary = vs.currentView.Primary
		s := vs.findIdleServer()
		if s != "" {
			vs.clients[s] = BUSY

			view.Primary = vs.currentView.Primary
			view.Backup = s
			view.Viewnum = vs.currentView.Viewnum + 1

			if backup != "" {
				vs.currentView.Backup = backup
				vs.previousView = vs.currentView
			}

			vs.currentView = view
			if !once {
				vs.acked = false
			}
			once = false
		}
	}

}

//
// server Ping RPC handler.
//
func (vs *ViewServer) Ping(args *PingArgs, reply *PingReply) error {

	// Your code here.
	clientName := args.Me
	// clients connected.
	defer func() {
		vs.lastViewNum[args.Me] = args.Viewnum
	}()
	defer func() {
		log.Println("_______++_______")
		log.Println("get ping", args.Me, "view num", args.Viewnum)
		log.Println("last current view", vs.currentView)
		log.Println("last previous view", vs.previousView)
		log.Println("_______--_______")
	}()
	vs.mu.Lock()
	defer vs.mu.Unlock()
	defer func() { vs.deadCount[clientName] = DeadPings }()
	// clients connected
	if vs.clients[clientName] != DEAD {
		vs.deadCount[clientName] = DeadPings
		// clients connected but restarted.
		// TODO: 记录ping的列表.
		if args.Viewnum < vs.lastViewNum[args.Me] && vs.clients[clientName] != DEAD {
			log.Println("restart connect")
			var change bool
			var (
				primary string
				backup  string
			)
			if args.Me == vs.currentView.Primary {
				primary = vs.currentView.Primary
				vs.currentView.Primary = ""
				vs.clients[args.Me] = IDLE
				change = true
			}
			if args.Me == vs.currentView.Backup {
				backup = vs.currentView.Backup
				vs.currentView.Backup = ""
				vs.clients[args.Me] = IDLE
				change = true
			}
			if change {
				vs.RerangePB(primary, backup)
			}
		} else {
			// 向现在的primary确定,可能backup取代了primary.
			// 但是之前的primary不应答不会向前提升.
			if args.Viewnum == vs.previousView.Viewnum && (args.Me == vs.previousView.Primary) {
				vs.acked = true
			}
		}
	} else {
		log.Println("new connect")
		// clients not connected.
		vs.clients[clientName] = IDLE
		vs.RerangePB(vs.currentView.Primary, vs.currentView.Backup)
	}
	reply.View = vs.currentView
	return nil
}

//
// server Get() RPC handler.
//
func (vs *ViewServer) Get(args *GetArgs, reply *GetReply) error {

	// Your code here.
	if vs.acked {
		reply.View = vs.currentView
	} else {
		reply.View = vs.previousView
	}
	log.Println("get view ", reply.View)
	return nil
}

//
// tick() is called once per PingInterval; it should notice
// if servers have died or recovered, and change the view
// accordingly.
//
func (vs *ViewServer) tick() {

	// Your code here.
	vs.mu.Lock()
	defer vs.mu.Unlock()
	for name, count := range vs.deadCount {
		if count <= 0 && vs.clients[name] != DEAD {
			// update view if the client is in the view.
			vs.clients[name] = DEAD
			log.Println(name + " dead.")
			if vs.currentView.Primary == name {
				var primary = vs.currentView.Primary
				vs.currentView.Primary = ""
				vs.RerangePB(primary, "")
			}
			if vs.currentView.Backup == name {
				var backup = vs.currentView.Backup
				vs.currentView.Backup = ""
				vs.RerangePB("", backup)
			}
		} else {
			vs.deadCount[name] = count - 1
		}
	}
}

//
// tell the server to shut itself down.
// for testing.
// please don't change these two functions.
//
func (vs *ViewServer) Kill() {
	atomic.StoreInt32(&vs.dead, 1)
	vs.l.Close()
}

//
// has this server been asked to shut down?
//
func (vs *ViewServer) isdead() bool {
	return atomic.LoadInt32(&vs.dead) != 0
}

// please don't change this function.
func (vs *ViewServer) GetRPCCount() int32 {
	return atomic.LoadInt32(&vs.rpccount)
}

func StartServer(me string) *ViewServer {
	vs := new(ViewServer)
	vs.me = me
	// Your vs.* initializations here.
	vs.acked = true
	vs.clients = make(map[string]State)
	vs.deadCount = make(map[string]int64)
	vs.lastViewNum = make(map[string]uint)
	// tell net/rpc about our RPC server and handlers.
	rpcs := rpc.NewServer()
	rpcs.Register(vs)

	// prepare to receive connections from clients.
	// change "unix" to "tcp" to use over a network.
	os.Remove(vs.me) // only needed for "unix"
	l, e := net.Listen("unix", vs.me)
	if e != nil {
		log.Fatal("listen error: ", e)
	}
	vs.l = l

	// please don't change any of the following code,
	// or do anything to subvert it.

	// create a thread to accept RPC connections from clients.
	go func() {
		for vs.isdead() == false {
			conn, err := vs.l.Accept()
			if err == nil && vs.isdead() == false {
				atomic.AddInt32(&vs.rpccount, 1)
				go rpcs.ServeConn(conn)
			} else if err == nil {
				conn.Close()
			}
			if err != nil && vs.isdead() == false {
				fmt.Printf("ViewServer(%v) accept: %v\n", me, err.Error())
				vs.Kill()
			}
		}
	}()

	// create a thread to call tick() periodically.
	go func() {
		for vs.isdead() == false {
			vs.tick()
			time.Sleep(PingInterval)
		}
	}()

	return vs
}
