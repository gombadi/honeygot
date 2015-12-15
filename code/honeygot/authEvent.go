package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

type AuthEvent struct {
	Time        string // number of seconds from Unix epoch
	AuthType    string // type of event - eg sshPass, sshKey
	SrcIP       string //
	DestIP      string //
	User        string // username used
	Credentials string // ssh password or ssh key used
	TypeData    string // extra data specific to the auth type. May be json and/or base64 encoded
	Hash        string // mostly uniq hash of the event
}

func (ae *AuthEvent) updateHash() {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s%s%s%s%s%s%v", ae.Time, ae.AuthType, ae.SrcIP, ae.DestIP, ae.User, ae.Credentials, ae.TypeData)))
	ae.Hash = base64.StdEncoding.EncodeToString(h.Sum(nil))
}
