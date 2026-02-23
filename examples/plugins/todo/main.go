package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/extism/go-pdk"
)

type todoInput struct {
	Action string `json:"action"`
	Text   string `json:"text"`
	ID     string `json:"id"`
}

type todoItem struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at"`
}

type todoList struct {
	Items  []todoItem `json:"items"`
	NextID int        `json:"next_id"`
}

const kvKey = "todos"

func loadTodos() *todoList {
	data := pdk.GetVar(kvKey)
	if data == nil {
		return &todoList{NextID: 1}
	}
	var list todoList
	if err := json.Unmarshal(data, &list); err != nil {
		return &todoList{NextID: 1}
	}
	return &list
}

func saveTodos(list *todoList) {
	data, _ := json.Marshal(list)
	pdk.SetVar(kvKey, data)
}

//export handle
func handle() int32 {
	input := pdk.Input()

	var req todoInput
	if err := json.Unmarshal(input, &req); err != nil {
		return outputJSON(map[string]string{"error": "invalid input: " + err.Error()})
	}

	switch req.Action {
	case "add":
		return handleAdd(req)
	case "list":
		return handleList()
	case "done":
		return handleDone(req)
	case "remove":
		return handleRemove(req)
	default:
		return outputJSON(map[string]string{"error": fmt.Sprintf("unknown action: %s", req.Action)})
	}
}

func handleAdd(req todoInput) int32 {
	if req.Text == "" {
		return outputJSON(map[string]string{"error": "text is required for add action"})
	}

	list := loadTodos()
	item := todoItem{
		ID:        strconv.Itoa(list.NextID),
		Text:      req.Text,
		Done:      false,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	list.Items = append(list.Items, item)
	list.NextID++
	saveTodos(list)

	return outputJSON(map[string]any{
		"status": "added",
		"item":   item,
	})
}

func handleList() int32 {
	list := loadTodos()
	return outputJSON(map[string]any{
		"items": list.Items,
		"count": len(list.Items),
	})
}

func handleDone(req todoInput) int32 {
	if req.ID == "" {
		return outputJSON(map[string]string{"error": "id is required for done action"})
	}

	list := loadTodos()
	for i := range list.Items {
		if list.Items[i].ID == req.ID {
			list.Items[i].Done = true
			saveTodos(list)
			return outputJSON(map[string]any{
				"status": "completed",
				"item":   list.Items[i],
			})
		}
	}
	return outputJSON(map[string]string{"error": fmt.Sprintf("task %s not found", req.ID)})
}

func handleRemove(req todoInput) int32 {
	if req.ID == "" {
		return outputJSON(map[string]string{"error": "id is required for remove action"})
	}

	list := loadTodos()
	for i := range list.Items {
		if list.Items[i].ID == req.ID {
			removed := list.Items[i]
			list.Items = append(list.Items[:i], list.Items[i+1:]...)
			saveTodos(list)
			return outputJSON(map[string]any{
				"status": "removed",
				"item":   removed,
			})
		}
	}
	return outputJSON(map[string]string{"error": fmt.Sprintf("task %s not found", req.ID)})
}

func outputJSON(v any) int32 {
	data, _ := json.Marshal(v)
	pdk.Output(data)
	return 0
}

func main() {}
