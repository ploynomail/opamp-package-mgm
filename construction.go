package opamppackagemgm

import (
	"context"
)

func NewUpdater(
	ctx context.Context,
	currentVersion string,
	tempDir string,
) *Updater {
	u := &Updater{
		ctx:            ctx,
		CurrentVersion: currentVersion,
		Dir:            tempDir,
		Requester:      NewHTTPRequester(),
	}
	return u
}

func (u *Updater) WithRequester(r Requester) *Updater {
	u.Requester = r
	return u
}

func (u *Updater) WithTrigger(t TriggerUpdater) *Updater {
	u.Trigger = t
	return u
}

func (u *Updater) WithLogger(l Loggerr) *Updater {
	u.Logger = l
	return u
}

func (u *Updater) WithOnSuccessfulUpdate(f func(context.Context)) *Updater {
	u.OnSuccessfulUpdate = f
	return u
}

func (u *Updater) WithOnFailedUpdate(f func(context.Context)) *Updater {
	u.OnFailedUpdate = f
	return u
}

func (u *Updater) WithIsGzipped(b bool) *Updater {
	u.IsGzipped = b
	return u
}
