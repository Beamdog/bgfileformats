package bg

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
)

type dlgHeader struct {
	Signature, Version      [4]byte
	StateCount              uint32
	StateOffset             uint32
	TransitionCount         uint32
	TransitionOffset        uint32
	StateTriggerOffset      uint32
	StateTriggerCount       uint32
	TransitionTriggerOffset uint32
	TransitionTriggerCount  uint32
	ActionOffset            uint32
	ActionCount             uint32
	Flags                   uint32
}

type dlgState struct {
	Stringref       uint32
	TransitionIndex uint32
	TransitionCount uint32
	TriggerIndex    int32
}

type dlgTransition struct {
	Flags                  uint32
	TransitionText         uint32
	JournalText            uint32
	TransitionTriggerIndex uint32
	TransitionActionIndex  uint32
	NextDlg                RESREF
	NextState              uint32
}

type dlgOffsetLength struct {
	Offset uint32
	Length uint32
}

type DLG struct {
	Header             dlgHeader
	States             []dlgState
	Transitions        []dlgTransition
	StateTriggers      []string
	TransitionTriggers []string
	Actions            []string
}

func (trans *dlgTransition) HasText() bool {
	return trans.Flags&0x0001 == 0x0001
}
func (trans *dlgTransition) HasTrigger() bool {
	return trans.Flags&0x0002 == 0x0002
}
func (trans *dlgTransition) HasAction() bool {
	return trans.Flags&0x0004 == 0x0004
}
func (trans *dlgTransition) TerminatesDialog() bool {
	return trans.Flags&0x0008 == 0x0008
}
func (trans *dlgTransition) HasJournal() bool {
	return trans.Flags&0x0010 == 0x0010
}
func (trans *dlgTransition) AddQuest() bool {
	return trans.Flags&0x0040 == 0x0040
}
func (trans *dlgTransition) RemoveQuest() bool {
	return trans.Flags&0x0080 == 0x0080
}
func (trans *dlgTransition) AddCompleteQuest() bool {
	return trans.Flags&0x0100 == 0x0100
}

/*
func (dlg *DLG) Print(tlk *TLK) {
	for idx, state := range dlg.States {
		str, _ := tlk.String(int(state.Stringref))
		fmt.Printf("State[%d]: %s\n", idx, str)
		if state.TriggerIndex < int32(len(dlg.StateTriggers)) && state.TriggerIndex >= 0 {
			fmt.Printf("Trigger: %#v\n", dlg.StateTriggers[state.TriggerIndex])
		}
		for _, transition := range dlg.Transitions[state.TransitionIndex : state.TransitionIndex + state.TransitionCount] {
			if transition.HasText() {
				str, _ := tlk.String(int(transition.TransitionText))
				fmt.Printf("\tText: %s\n", str)
			}
			if transition.HasTrigger() && transition.TransitionTriggerIndex < uint32(len(dlg.TransitionTriggers)) && transition.TransitionTriggerIndex >= 0 {
				fmt.Printf("\tTrigger: %#v\n", dlg.TransitionTriggers[transition.TransitionTriggerIndex])
			}
			if transition.HasAction() && transition.TransitionActionIndex < uint32(len(dlg.Actions)) {
				fmt.Printf("\t\tAction: %#v\n", dlg.Actions[transition.TransitionActionIndex])
			}
			if transition.HasJournal() {
				str, _ := tlk.String(int(transition.JournalText))
				fmt.Printf("\t\tJournal Entry: %s\n", str)
			}
			if transition.TerminatesDialog() {
				fmt.Printf("\t\tDialog Exit\n")
			} else {
				fmt.Printf("\t\tNext Dialog: %s [%d]\n", transition.NextDlg, transition.NextState)
			}
		}
	}
}

func (dlg *DLG) PrintDot(tlk *TLK, root string) {
	root = strings.ToLower(root)
	for idx, state := range dlg.States {
		//str, _ := tlk.String(int(state.Stringref))
		if state.TriggerIndex >= 0 {
			fmt.Printf("%s -> %s_s%d;\n", root, root, idx);
		}
		for _, transition := range dlg.Transitions[state.TransitionIndex : state.TransitionIndex + state.TransitionCount] {
			if !transition.TerminatesDialog() {
				//fmt.Printf("%s_s%d -> %s_s%d_t%d;\n", root, idx, root, idx, tIdx)
				fmt.Printf("%s_s%d -> %s_s%d;\n", root, idx, strings.ToLower(transition.NextDlg.String()), transition.NextState)
			}
		}
	}
}
*/

func OpenDlg(r io.ReadSeeker) (*DLG, error) {
	dlg := &DLG{}

	err := binary.Read(r, binary.LittleEndian, &dlg.Header)
	if err != nil {
		return nil, err
	}

	dlg.States = make([]dlgState, dlg.Header.StateCount)
	_, err = r.Seek(int64(dlg.Header.StateOffset), os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.LittleEndian, &dlg.States)
	if err != nil {
		return nil, err
	}

	dlg.Transitions = make([]dlgTransition, dlg.Header.TransitionCount)
	_, err = r.Seek(int64(dlg.Header.TransitionOffset), os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.LittleEndian, &dlg.Transitions)
	if err != nil {
		return nil, err
	}

	dlg.StateTriggers = make([]string, dlg.Header.StateTriggerCount)

	for idx := range dlg.StateTriggers {
		ol := dlgOffsetLength{}
		_, err = r.Seek(int64(int(dlg.Header.StateTriggerOffset)+idx*binary.Size(ol)), os.SEEK_SET)
		if err != nil {
			return nil, err
		}
		err = binary.Read(r, binary.LittleEndian, &ol)
		_, err = r.Seek(int64(ol.Offset), os.SEEK_SET)
		if err != nil {
			return nil, err
		}
		data := make([]byte, ol.Length)
		r.Read(data)
		dlg.StateTriggers[idx] = string(data[0:])
	}

	dlg.TransitionTriggers = make([]string, dlg.Header.TransitionTriggerCount)
	for idx := range dlg.TransitionTriggers {
		ol := dlgOffsetLength{}
		_, err = r.Seek(int64(int(dlg.Header.TransitionTriggerOffset)+idx*binary.Size(ol)), os.SEEK_SET)
		if err != nil {
			return nil, err
		}
		err = binary.Read(r, binary.LittleEndian, &ol)
		_, err = r.Seek(int64(ol.Offset), os.SEEK_SET)
		if err != nil {
			return nil, err
		}
		data := make([]byte, ol.Length)
		r.Read(data)
		dlg.TransitionTriggers[idx] = string(data[0:])
	}
	dlg.Actions = make([]string, dlg.Header.ActionCount)
	for idx := range dlg.Actions {
		ol := dlgOffsetLength{}
		_, err = r.Seek(int64(int(dlg.Header.ActionOffset)+idx*binary.Size(ol)), os.SEEK_SET)
		if err != nil {
			return nil, err
		}
		err = binary.Read(r, binary.LittleEndian, &ol)
		_, err = r.Seek(int64(ol.Offset), os.SEEK_SET)
		if err != nil {
			return nil, err
		}
		data := make([]byte, ol.Length)
		r.Read(data)
		dlg.Actions[idx] = string(data[0:])
	}

	return dlg, nil
}

func (dialog *DLG) WriteJson(w io.Writer) error {
	bytes, err := json.MarshalIndent(dialog, "", "\t")
	if err != nil {
		return err
	}

	_, err = w.Write(bytes)
	return err
}
