// Package components describe all components used by the TUI
package components

import tea "charm.land/bubbletea/v2"

type localZone struct{}

func (localZone) InBounds(tea.MouseMsg) bool { return false }

type localManager struct{}

func (localManager) Mark(id, content string) string { return content }
func (localManager) Scan(content string) string     { return content }
func (localManager) Get(id string) localZone        { return localZone{} }
func (localManager) NewPrefix() string              { return "" }
func (localManager) NewGlobal()                     {}
func (localManager) Close()                         {}

var Z = localManager{}
