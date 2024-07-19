/*
Copyright (c) 2024-2024 VMware, Inc. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package check

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/govc/flags"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/vim25/xml"
)

type checkFlag struct {
	*flags.VirtualMachineFlag
	*flags.HostSystemFlag
	*flags.ResourcePoolFlag

	Machine, Host, Pool *types.ManagedObjectReference

	Test []string
}

func (cmd *checkFlag) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.VirtualMachineFlag, ctx = flags.NewVirtualMachineFlag(ctx)
	cmd.VirtualMachineFlag.Register(ctx, f)
	cmd.HostSystemFlag, ctx = flags.NewHostSystemFlag(ctx)
	cmd.HostSystemFlag.Register(ctx, f)
	cmd.ResourcePoolFlag, ctx = flags.NewResourcePoolFlag(ctx)
	cmd.ResourcePoolFlag.Register(ctx, f)
}

func (cmd *checkFlag) Process(ctx context.Context) error {
	if err := cmd.VirtualMachineFlag.Process(ctx); err != nil {
		return err
	}
	if err := cmd.HostSystemFlag.Process(ctx); err != nil {
		return err
	}
	if err := cmd.ResourcePoolFlag.Process(ctx); err != nil {
		return err
	}

	vm, err := cmd.VirtualMachine()
	if err != nil {
		return err
	}
	if vm != nil {
		cmd.Machine = types.NewReference(vm.Reference())
	}

	host, err := cmd.HostSystemIfSpecified()
	if err != nil {
		return err
	}
	if host != nil {
		cmd.Host = types.NewReference(host.Reference())
	}

	pool, err := cmd.ResourcePoolIfSpecified()
	if err != nil {
		return err
	}
	if pool != nil {
		cmd.Pool = types.NewReference(pool.Reference())
	}

	return nil
}

func (cmd *checkFlag) provChecker() (*object.VmProvisioningChecker, error) {
	c, err := cmd.VirtualMachineFlag.Client()
	if err != nil {
		return nil, err
	}

	return object.NewVmProvisioningChecker(c), nil
}

func (cmd *checkFlag) compatChecker() (*object.VmCompatibilityChecker, error) {
	c, err := cmd.VirtualMachineFlag.Client()
	if err != nil {
		return nil, err
	}

	return object.NewVmCompatibilityChecker(c), nil
}

func (cmd *checkFlag) Spec(spec any) error {
	dec := xml.NewDecoder(os.Stdin)
	dec.TypeFunc = types.TypeFunc()
	return dec.Decode(spec)
}

// return cmd.VirtualMachineFlag.WriteResult(&checkResult{res, ctx, cmd.VirtualMachineFlag})
func (cmd *checkFlag) result(ctx context.Context, res []types.CheckResult) error {
	return cmd.VirtualMachineFlag.WriteResult(&checkResult{res, ctx, cmd.VirtualMachineFlag})
}

type checkResult struct {
	Result []types.CheckResult `json:"result"`
	ctx    context.Context
	vm     *flags.VirtualMachineFlag
}

func (res *checkResult) Write(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)

	c, err := res.vm.Client()
	if err != nil {
		return err
	}

	for _, r := range res.Result {
		fields := []struct {
			name  string
			moid  *types.ManagedObjectReference
			fault []types.LocalizedMethodFault
		}{
			{"VM", r.Vm, nil},
			{"Host", r.Host, nil},
			{"Warning", nil, r.Warning},
			{"Error", nil, r.Error},
		}

		for _, f := range fields {
			var val string
			if f.moid == nil {
				var msgs []string
				for _, m := range f.fault {
					msgs = append(msgs, m.LocalizedMessage)
				}
				val = strings.Join(slices.Compact(msgs), "\n\t")
			} else {
				val, err = find.InventoryPath(res.ctx, c, *f.moid)
				if err != nil {
					return err
				}
			}
			fmt.Fprintf(tw, "%s:\t%s\n", f.name, val)
		}
	}

	return tw.Flush()
}
