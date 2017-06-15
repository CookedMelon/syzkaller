// Copyright 2017 syzkaller project authors. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

//go:generate bash -c "echo -en '// AUTOGENERATED FILE\n\n' > generated.go"
//go:generate bash -c "echo -en 'package kernel\n\n' >> generated.go"
//go:generate bash -c "echo -en 'const createImageScript = `#!/bin/bash\n' >> generated.go"
//go:generate bash -c "cat ../../tools/create-gce-image.sh | grep -v '#' >> generated.go"
//go:generate bash -c "echo -en '`\n\n' >> generated.go"

// Package kernel contains helper functions for working with Linux kernel
// (building kernel/image).
package kernel

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/syzkaller/pkg/osutil"
)

func Build(dir, compiler, config string, fullConfig bool) error {
	const timeout = 10 * time.Minute // default timeout for command invocations
	if fullConfig {
		if err := ioutil.WriteFile(filepath.Join(dir, ".config"), []byte(config), 0600); err != nil {
			return fmt.Errorf("failed to write config file: %v", err)
		}
	} else {
		os.Remove(filepath.Join(dir, ".config"))
		configFile := filepath.Join(dir, "syz.config")
		if err := ioutil.WriteFile(configFile, []byte(config), 0600); err != nil {
			return fmt.Errorf("failed to write config file: %v", err)
		}
		defer os.Remove(configFile)
		if _, err := osutil.RunCmd(timeout, dir, "make", "defconfig"); err != nil {
			return err
		}
		if _, err := osutil.RunCmd(timeout, dir, "make", "kvmconfig"); err != nil {
			return err
		}
		if _, err := osutil.RunCmd(timeout, dir, "scripts/kconfig/merge_config.sh", "-n", ".config", configFile); err != nil {
			return err
		}
	}
	if _, err := osutil.RunCmd(timeout, dir, "make", "olddefconfig"); err != nil {
		return err
	}
	// We build only bzImage as we currently don't use modules.
	// Build of a large kernel can take a while on a 1 CPU VM.
	if _, err := osutil.RunCmd(3*time.Hour, dir, "make", "bzImage", "-j", strconv.Itoa(runtime.NumCPU()*2), "CC="+compiler); err != nil {
		return err
	}
	return nil
}

// CreateImage creates a disk image that is suitable for syzkaller.
// Kernel is taken from kernelDir, userspace system is taken from userspaceDir.
// The resulting image is marked with tag and copied to the specified image file.
func CreateImage(kernelDir, userspaceDir, tag, image string) error {
	tempDir, err := ioutil.TempDir("", "syz-build")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	scriptFile := filepath.Join(tempDir, "create.sh")
	if err := ioutil.WriteFile(scriptFile, []byte(createImageScript), 0700); err != nil {
		return fmt.Errorf("failed to write script file: %v", err)
	}
	vmlinux := filepath.Join(kernelDir, "vmlinux")
	bzImage := filepath.Join(kernelDir, "arch/x86/boot/bzImage")
	if _, err := osutil.RunCmd(time.Hour, tempDir, scriptFile, userspaceDir, bzImage, vmlinux, tag); err != nil {
		return fmt.Errorf("image build failed: %v", err)
	}
	if err := os.Rename(filepath.Join(tempDir, "image.tar.gz"), image); err != nil {
		return err
	}
	return nil
}

func CompilerIdentity(compiler string) (string, error) {
	output, err := osutil.RunCmd(time.Minute, "", compiler, "--version")
	if err != nil {
		return "", err
	}
	if len(output) == 0 {
		return "", fmt.Errorf("no output from compiler --version")
	}
	return strings.Split(string(output), "\n")[0], nil
}
