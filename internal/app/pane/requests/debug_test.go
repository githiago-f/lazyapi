package requests

import (
	"fmt"
	"testing"
)

func TestDebugRenderAll(t *testing.T) {
	fmt.Println("=== REQUEST LIST (normal) ===")
	fmt.Println(DebugRenderList(50, 14))
	fmt.Println()
	fmt.Println("=== REQUEST LIST (filtered 'pet') ===")
	fmt.Println(DebugRenderListFiltered(50, 10))
	fmt.Println()
	fmt.Println("=== TAGS OVERLAY ===")
	fmt.Println(DebugRenderTagsOverlay(40))
}

func TestDebugCursorMovement(t *testing.T) {
	fmt.Println("=== CURSOR MOVEMENT (each step) ===")
	fmt.Println(DebugCursorMovement())
}
