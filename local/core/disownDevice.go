package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/plgd-dev/sdk/schema"
)

func connectionWasClosed(ctx context.Context, err error) bool {
	if ctx.Err() == nil && errors.Is(err, context.Canceled) {
		return true
	}
	return false
}

// DisownDevice remove ownership of device
func (d *Device) Disown(
	ctx context.Context,
	links schema.ResourceLinks,
) error {
	const errMsg = "cannot disown: %w"

	ownership, err := d.GetOwnership(ctx, links)
	if err != nil {
		return fmt.Errorf(errMsg, err)
	}

	sdkID, err := d.GetSdkOwnerID()
	if err != nil {
		return fmt.Errorf(errMsg, err)
	}

	if ownership.OwnerID != sdkID {
		if ownership.OwnerID == uuid.Nil.String() {
			return nil
		}
		return MakePermissionDenied(fmt.Errorf(errMsg, fmt.Errorf("device is owned by %v, not by %v", ownership.OwnerID, sdkID)))
	}

	setResetProvisionState := schema.ProvisionStatusUpdateRequest{
		DeviceOnboardingState: &schema.DeviceOnboardingState{
			CurrentOrPendingOperationalState: schema.OperationalState_RESET,
		},
	}

	link, err := GetResourceLink(links, "/oic/sec/pstat")
	if err != nil {
		return MakeInternal(fmt.Errorf(errMsg, err))
	}
	link.Endpoints = link.Endpoints.FilterSecureEndpoints()

	err = d.UpdateResource(ctx, link, setResetProvisionState, nil)
	if err != nil {
		if connectionWasClosed(ctx, err) {
			// connection was closed by disown so we don't report error just log it.
			d.cfg.errFunc(fmt.Errorf(errMsg, err))
			return nil
		}

		return MakeInternal(fmt.Errorf(errMsg, err))
	}

	return nil
}
