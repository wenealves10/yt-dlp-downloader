package sse

import (
	"testing"
	"time"
)

func TestSSEManager_BasicPublishSubscribe(t *testing.T) {
	manager := NewSSEManager()
	id := "basic-test"

	sub := manager.Subscribe(id)
	defer manager.Unsubscribe(id, sub)

	go func() {
		time.Sleep(50 * time.Millisecond)
		manager.Publish(id, "test-message")
	}()

	select {
	case msg := <-sub:
		if msg != "test-message" {
			t.Errorf("esperado 'test-message', recebido '%s'", msg)
		}
	case <-time.After(time.Second):
		t.Error("timeout esperando mensagem")
	}
}

func TestSSEManager_MultipleSubscribers(t *testing.T) {
	manager := NewSSEManager()
	id := "multi-sub"

	sub1 := manager.Subscribe(id)
	sub2 := manager.Subscribe(id)
	defer manager.Unsubscribe(id, sub1)
	defer manager.Unsubscribe(id, sub2)

	manager.Publish(id, "broadcast")

	for i, sub := range []Subscriber{sub1, sub2} {
		select {
		case msg := <-sub:
			if msg != "broadcast" {
				t.Errorf("esperado 'broadcast', recebido '%s'", msg)
			}
		case <-time.After(time.Second):
			t.Errorf("timeout esperando mensagem para sub %d", i+1)
		}
	}
}

func TestSSEManager_Unsubscribe(t *testing.T) {
	manager := NewSSEManager()
	id := "unsub-test"

	sub := manager.Subscribe(id)
	manager.Unsubscribe(id, sub)

	manager.Publish(id, "after-unsub")

	select {
	case _, ok := <-sub:
		if ok {
			t.Error("esperado canal fechado após unsubscribe, mas ainda está aberto")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("esperado canal fechado, mas bloqueou")
	}
}

func TestSSEManager_EmptySubscribers(t *testing.T) {
	manager := NewSSEManager()
	id := "no-subs"

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic ao publicar sem subscribers: %v", r)
		}
	}()

	manager.Publish(id, "no-subscriber-message")
}

func TestSSEManager_DoubleUnsubscribe(t *testing.T) {
	manager := NewSSEManager()
	id := "double-unsub"

	sub := manager.Subscribe(id)
	manager.Unsubscribe(id, sub)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic ao tentar Unsubscribe duas vezes: %v", r)
		}
	}()

	manager.Unsubscribe(id, sub)
}
