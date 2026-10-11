//go:build linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type recoveryNode struct {
	name, config, log string
	cmd               *exec.Cmd
}

// This test never accepts an external broker URL. All client/route listeners,
// process handles and stores belong to this test's disposable cluster.
func TestGenerationThreeNodeRecovery(t *testing.T) {
	bin := os.Getenv("WORLDBUILDER_TEST_NATS_SERVER_BIN")
	if bin == "" {
		t.Skip("run scripts/worldbuilder/test-jetstream.sh for disposable three-node acceptance")
	}
	dir := t.TempDir()
	nodes := make([]recoveryNode, 3)
	ports := make([]int, 6)
	var reservations []net.Listener
	for i := range ports {
		l, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		reservations = append(reservations, l)
		ports[i] = l.Addr().(*net.TCPAddr).Port
	}
	for _, l := range reservations {
		l.Close()
	}
	stop := func(i int) {
		if nodes[i].cmd != nil {
			// Abrupt process failure, not graceful leader transfer.
			_ = nodes[i].cmd.Process.Kill()
			_ = nodes[i].cmd.Wait()
			nodes[i].cmd = nil
		}
	}
	t.Cleanup(func() {
		for i := range nodes {
			stop(i)
			if t.Failed() {
				b, _ := os.ReadFile(nodes[i].log)
				if len(b) > 6000 {
					b = b[len(b)-6000:]
				}
				t.Logf("%s server log:\n%s", nodes[i].name, b)
			}
		}
	})
	start := func(i int) {
		f, err := os.OpenFile(nodes[i].log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		cmd := exec.Command(bin, "-c", nodes[i].config)
		// Even a test timeout or killed test binary must not leave brokers running.
		cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
		cmd.Stdout, cmd.Stderr = f, f
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		nodes[i].cmd = cmd
	}
	var urls []string
	for i := range nodes {
		nodes[i] = recoveryNode{name: fmt.Sprintf("offline-%d", i), config: filepath.Join(dir, fmt.Sprintf("node-%d.conf", i)), log: filepath.Join(dir, fmt.Sprintf("node-%d.log", i))}
		var routes []string
		for j := range nodes {
			if j != i {
				routes = append(routes, fmt.Sprintf("\"nats://127.0.0.1:%d\"", ports[j+3]))
			}
		}
		config := fmt.Sprintf("server_name: %q\nlisten: 127.0.0.1:%d\njetstream { store_dir: %q, max_memory_store: 16MB, max_file_store: 64MB }\ncluster { name: offline-recovery, listen: 127.0.0.1:%d, routes: [%s] }\n", nodes[i].name, ports[i], filepath.Join(dir, fmt.Sprintf("store-%d", i)), ports[i+3], strings.Join(routes, ","))
		if err := os.WriteFile(nodes[i].config, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
		urls = append(urls, fmt.Sprintf("nats://127.0.0.1:%d", ports[i]))
		start(i)
	}
	wait := func(label string, check func() error) {
		t.Helper()
		deadline := time.Now().Add(40 * time.Second)
		var err error
		for time.Now().Before(deadline) {
			if err = check(); err == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatalf("%s: %v", label, err)
	}
	var nc *nats.Conn
	wait("loopback broker startup", func() (err error) {
		nc, err = nats.Connect(strings.Join(urls, ","), nats.Timeout(time.Second), nats.ReconnectWait(100*time.Millisecond), nats.MaxReconnects(-1))
		return err
	})
	defer nc.Close()
	js, err := nc.JetStream(nats.MaxWait(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	wait("three-replica file stream", func() error { return ensureGenerationStream(js) })
	healthy := func(replicas int) error {
		info, e := js.StreamInfo(generationStream)
		if e != nil {
			return e
		}
		if info.Cluster == nil || info.Cluster.Leader == "" || info.Config.Replicas != 3 {
			return fmt.Errorf("missing three-replica leader: %+v", info.Cluster)
		}
		current := 1
		for _, r := range info.Cluster.Replicas {
			if r.Current && !r.Offline {
				current++
			}
		}
		if current < replicas {
			return fmt.Errorf("only %d current replicas, need %d", current, replicas)
		}
		return nil
	}
	wait("all replicas caught up", func() error { return healthy(3) })
	const durable = "worldbuilder-generation-v1"
	sub, err := subscribeGeneration(js)
	if err != nil {
		t.Fatal(err)
	}
	initialConsumer, err := js.ConsumerInfo(generationStream, durable)
	if err != nil || initialConsumer.Config.AckWait != 2*time.Minute || initialConsumer.Config.MaxAckPending != 10 {
		t.Fatalf("production consumer contract: %+v %v", initialConsumer, err)
	}
	// Compress only the redelivery timer; retain production durability/bounds.
	fastConsumer := initialConsumer.Config
	fastConsumer.AckWait = time.Second
	if _, err = js.UpdateConsumer(generationStream, &fastConsumer); err != nil {
		t.Fatal(err)
	}
	consumerHealthy := func() error {
		c, e := js.ConsumerInfo(generationStream, durable)
		if e != nil {
			return e
		}
		if c.Config.Replicas != 3 || c.Config.MemoryStorage || c.Cluster == nil || len(c.Cluster.Replicas) != 2 {
			return fmt.Errorf("consumer is not replicated on disk: %+v", c)
		}
		for _, r := range c.Cluster.Replicas {
			if !r.Current || r.Offline {
				return fmt.Errorf("consumer replica not current: %+v", r)
			}
		}
		return nil
	}
	wait("replicated durable consumer", consumerHealthy)
	event := generationEvent{uuid.NewString(), uuid.NewString(), uuid.NewString(), 1}
	body, _ := json.Marshal(event)
	msg := nats.NewMsg(generationSubject)
	msg.Data = body
	msg.Header.Set("Nats-Msg-Id", event.EventID)
	first, err := js.PublishMsg(msg)
	if err != nil || first.Duplicate {
		t.Fatalf("initial publish: %+v %v", first, err)
	}
	messages, err := sub.Fetch(1, nats.MaxWait(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	token := strings.Repeat("k", 32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ref map[string]string
		if r.Method != "POST" || r.URL.Path != "/api/worldbuilder-jobs/run" || r.Header.Get("Authorization") != "Bearer "+token || json.NewDecoder(r.Body).Decode(&ref) != nil || len(ref) != 2 || ref["worldId"] != event.WorldID || ref["jobId"] != event.JobID {
			t.Error("worker request lost its private saved-intent boundary")
			http.Error(w, "invalid reference", 400)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"done": calls.Add(1) > 1})
	}))
	defer server.Close()
	if done, e := runGenerationEvent(context.Background(), messages[0].Data, server.URL, token, server.Client()); e != nil || done {
		t.Fatalf("initial incomplete checkpoint: %t %v", done, e)
	}
	info, err := js.StreamInfo(generationStream)
	if err != nil {
		t.Fatal(err)
	}
	leader := -1
	for i := range nodes {
		if nodes[i].name == info.Cluster.Leader {
			leader = i
		}
	}
	if leader < 0 {
		t.Fatal("stream leader is outside the disposable cluster")
	}
	t.Logf("kill stream leader %s with one unacknowledged dispatch", nodes[leader].name)
	stop(leader)
	wait("stream leader failover with majority", func() error {
		if e := healthy(2); e != nil {
			return e
		}
		i, e := js.StreamInfo(generationStream)
		if e == nil && i.Cluster.Leader == nodes[leader].name {
			return fmt.Errorf("old leader still advertised")
		}
		return e
	})
	var duplicate *nats.PubAck
	wait("stable dispatch ID survives failover", func() (e error) { duplicate, e = js.PublishMsg(msg); return e })
	if !duplicate.Duplicate || duplicate.Sequence != first.Sequence {
		t.Fatalf("dispatch duplicated after leader failure: %+v", duplicate)
	}
	wait("unacknowledged dispatch redelivered", func() (e error) { messages, e = sub.Fetch(1, nats.MaxWait(time.Second)); return e })
	meta, err := messages[0].Metadata()
	if err != nil || meta.NumDelivered < 2 || meta.Sequence.Stream != first.Sequence || string(messages[0].Data) != string(body) {
		t.Fatalf("incorrect redelivery: %+v %v", meta, err)
	}
	if done, e := runGenerationEvent(context.Background(), messages[0].Data, server.URL, token, server.Client()); e != nil || !done || calls.Load() != 2 {
		t.Fatalf("resume saved job reference: %t %v calls=%d", done, e, calls.Load())
	}
	if err = messages[0].AckSync(); err != nil {
		t.Fatal(err)
	}
	// Remove a second server: one replica must never acknowledge a new publish.
	second := (leader + 1) % 3
	stop(second)
	t.Log("two nodes stopped: require publish refusal without quorum")
	uncertain := nats.NewMsg(generationSubject)
	uncertain.Data = []byte(`{"fixture":"quorum-loss"}`)
	uncertain.Header.Set("Nats-Msg-Id", uuid.NewString())
	if ack, e := js.PublishMsg(uncertain, nats.AckWait(2*time.Second)); e == nil {
		t.Fatalf("minority acknowledged publication: %+v", ack)
	}
	// A timed-out publish can still commit during recovery. Retry the SAME ID,
	// then count stored messages; do not assume a timeout proves non-delivery.
	start(second)
	start(leader)
	wait("three nodes restored", func() error { return healthy(3) })
	var recovered *nats.PubAck
	wait("uncertain publish reconciled by stable ID", func() (e error) { recovered, e = js.PublishMsg(uncertain); return e })
	if recovered.Sequence != first.Sequence+1 {
		t.Fatalf("uncertain publish changed sequence: %+v", recovered)
	}
	wait("post-quorum message delivered", func() (e error) { messages, e = sub.Fetch(1, nats.MaxWait(time.Second)); return e })
	if string(messages[0].Data) != string(uncertain.Data) {
		t.Fatal("acknowledged first dispatch was redelivered")
	}
	if err = messages[0].AckSync(); err != nil {
		t.Fatal(err)
	}
	wait("consumer replicas catch up before restart", consumerHealthy)
	wait("durable acknowledgement floor", func() error {
		c, e := js.ConsumerInfo(generationStream, durable)
		if e == nil && (c.NumAckPending != 0 || c.AckFloor.Stream != recovered.Sequence) {
			return fmt.Errorf("acknowledgement not durable: %+v", c)
		}
		return e
	})
	t.Log("restart all three processes from their existing file stores")
	for i := range nodes {
		stop(i)
	}
	for i := range nodes {
		start(i)
	}
	wait("full cluster restart", func() error { return healthy(3) })
	wait("consumer restart", consumerHealthy)
	info, err = js.StreamInfo(generationStream)
	if err != nil || info.State.Msgs != 2 {
		t.Fatalf("durable messages lost or duplicated: %+v %v", info, err)
	}
	for seq, want := range map[uint64]string{first.Sequence: string(body), recovered.Sequence: string(uncertain.Data)} {
		stored, e := js.GetMsg(generationStream, seq)
		if e != nil || string(stored.Data) != want {
			t.Fatalf("restored message %d changed: %+v %v", seq, stored, e)
		}
	}
	ci, err := js.ConsumerInfo(generationStream, durable)
	if err != nil || ci.NumAckPending != 0 || ci.NumPending != 0 || ci.AckFloor.Stream != recovered.Sequence {
		t.Fatalf("restored consumer lost acknowledgements: %+v %v", ci, err)
	}
	if _, err = sub.Fetch(1, nats.MaxWait(time.Second)); err != nats.ErrTimeout {
		t.Fatalf("acknowledged work redelivered after restart: %v", err)
	}
	ack, err := js.PublishMsg(msg)
	if err != nil || !ack.Duplicate || ack.Sequence != first.Sequence {
		t.Fatalf("restart lost deduplication identity: %+v %v", ack, err)
	}
	// Configuration drift is refused without silently rewriting existing data.
	wrongStream := info.Config
	wrongStream.MaxAge = time.Hour
	wait("metadata leader accepts retention fixture", func() error { _, e := js.UpdateStream(&wrongStream); return e })
	if err = ensureGenerationStream(js); err == nil || err.Error() != "incompatible generation stream" {
		t.Fatalf("expected retention refusal: %v", err)
	}
	unchanged, err := js.StreamInfo(generationStream)
	if err != nil || unchanged.Config.MaxAge != time.Hour || unchanged.State.Msgs != 2 {
		t.Fatalf("drift refusal mutated stream: %+v %v", unchanged, err)
	}
	wait("remove disposable consumer", func() error {
		e := js.DeleteConsumer(generationStream, durable)
		if e == nats.ErrConsumerNotFound {
			return nil
		}
		return e
	})
	weakConsumer := initialConsumer.Config
	weakConsumer.Replicas = 1
	wait("single-replica consumer fixture", func() error { _, e := js.AddConsumer(generationStream, &weakConsumer); return e })
	if _, err = subscribeGeneration(js); err == nil || !strings.Contains(err.Error(), "replicas") {
		t.Fatalf("expected consumer replica refusal: %v", err)
	}
	t.Log("PASS: leader failover, durable redelivery, minority refusal, uncertain retry, full restart and acknowledgement persistence")
}
