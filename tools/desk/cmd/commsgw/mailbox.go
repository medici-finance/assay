package main

import (
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
)

// mailbox.go — thin aliases onto internal/commsqueue, the on-disk shape
// SHARED with ../commsloop (a separate process/binary that drains the same
// accepted-queue this gateway writes — see commsqueue's package doc for why
// the shape lives there and not duplicated in each binary).

type AcceptedItem = commsqueue.AcceptedItem
type HeldItem = commsqueue.HeldItem
type Notice = commsqueue.Notice

func WriteAccepted(root string, env comms.Envelope, now time.Time) error {
	return commsqueue.WriteAccepted(root, env, now)
}

func ListAccepted(root string) ([]AcceptedItem, error) { return commsqueue.ListAccepted(root) }

func RemoveAccepted(root, id string) error { return commsqueue.RemoveAccepted(root, id) }

func WriteHeld(root string, env comms.Envelope, reason string, now time.Time) error {
	return commsqueue.WriteHeld(root, env, reason, now)
}

func ListHeld(root string) ([]HeldItem, error) { return commsqueue.ListHeld(root) }

func DeliverToMailbox(root, cell, role string, notice Notice) error {
	return commsqueue.DeliverToMailbox(root, cell, role, notice)
}

func PollMailbox(root, cell, role string) ([]Notice, error) {
	return commsqueue.PollMailbox(root, cell, role)
}

func AckMailbox(root, cell, role, id string) error {
	return commsqueue.AckMailbox(root, cell, role, id)
}
