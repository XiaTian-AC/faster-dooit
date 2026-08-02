package model

import (
	"testing"
	"time"
)

func TestTodoStatus(t *testing.T) {
	t.Run("pending", func(t *testing.T) {
		todo := &Todo{Pending: true}
		if got := todo.Status(); got != "pending" {
			t.Errorf("Status() = %q, want pending", got)
		}
	})
	t.Run("completed", func(t *testing.T) {
		todo := &Todo{Pending: false}
		if got := todo.Status(); got != "completed" {
			t.Errorf("Status() = %q, want completed", got)
		}
	})
	t.Run("overdue", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		todo := &Todo{Pending: true, Due: &past}
		if got := todo.Status(); got != "overdue" {
			t.Errorf("Status() = %q, want overdue", got)
		}
	})
}

func TestTodoTags(t *testing.T) {
	todo := &Todo{Description: "buy milk @grocery @errand"}
	got := todo.Tags()
	if len(got) != 2 || got[0] != "@grocery" || got[1] != "@errand" {
		t.Errorf("Tags() = %v", got)
	}
}
