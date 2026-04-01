//go:build android && !cmfa

package sing_tun

import (
	"errors"
	"sync"

	"github.com/metacubex/mihomo/component/process"
	"github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/constant/features"
	"github.com/metacubex/mihomo/log"

	"github.com/metacubex/sing-tun"
)

type packageManagerCallback struct{}

func (cb *packageManagerCallback) OnPackagesUpdated(packageCount int, sharedCount int) {}

func newPackageManager() (tun.PackageManager, error) {
	packageManager, err := tun.NewPackageManager(tun.PackageManagerOptions{
		Callback: &packageManagerCallback{},
		Logger:   log.SingLogger,
	})
	if err != nil {
		return nil, err
	}
	err = packageManager.Start()
	if err != nil {
		return nil, err
	}
	return packageManager, nil
}

var (
	globalPM tun.PackageManager
	pmOnce   sync.Once
	pmErr    error
)

func getPackageManager() (tun.PackageManager, error) {
	pmOnce.Do(func() {
		globalPM, pmErr = newPackageManager()
	})
	return globalPM, pmErr
}

func hasAndroidRules(tunOptions *tun.Options) bool {
	return len(tunOptions.IncludeAndroidUser) > 0 ||
		len(tunOptions.IncludePackage) > 0 ||
		len(tunOptions.ExcludePackage) > 0
}

func needsPackageManager(tunOptions *tun.Options) bool {
	return len(tunOptions.IncludePackage) > 0 ||
		len(tunOptions.ExcludePackage) > 0
}

func (l *Listener) buildAndroidRules(tunOptions *tun.Options) error {
	if !hasAndroidRules(tunOptions) {
		return nil
	}

	if !needsPackageManager(tunOptions) {
		tunOptions.BuildAndroidRules(nil, l.handler)
		return nil
	}

	packageManager, err := getPackageManager()
	if err != nil {
		log.Warnln("[Android TUN] initialize package manager failed, skip package rules: %v", err)
		tunOptions.IncludePackage = nil
		tunOptions.ExcludePackage = nil
		tunOptions.BuildAndroidRules(nil, l.handler)
		return nil
	}
	tunOptions.BuildAndroidRules(packageManager, l.handler)
	return nil
}

func findPackageName(metadata *constant.Metadata) (string, error) {
	packageManager, err := getPackageManager()
	if err != nil {
		return "", err
	}
	uid := metadata.Uid
	if sharedPackage, loaded := packageManager.SharedPackageByID(uid % 100000); loaded {
		return sharedPackage, nil
	}
	if packageName, loaded := packageManager.PackageByID(uid % 100000); loaded {
		return packageName, nil
	}
	return "", errors.New("package not found")
}

func init() {
	if !features.CMFA {
		process.DefaultPackageNameResolver = findPackageName
	}
}
