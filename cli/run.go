package cli

import (
	"fmt"
)

func Run(erase bool) {
	fmt.Println("sfconfig started with: ", GetConfigOrDie())
	if erase {
		fmt.Println("Erasing...")
		// TODO: remove the sfconfig resource and the pv
	} else {
		// TODO: if sf is already running then print info and stop
		// TODO: if kubectl is not connecting ask for reboot or rebuild
		// TODO: if microshift host is up but service is done, apply the ansible-microshift-role
		// TODO: install cert-manager if it is not available
		// TODO: install the crd if it is not available
		// TODO: install the cr if it is not available
		// TODO: run the operator if sf is not already running

		// TODO: suggest sfconfig --erase if the command does not succeed.
	}
}
