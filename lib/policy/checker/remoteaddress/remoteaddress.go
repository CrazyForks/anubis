package remoteaddress

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/lib/policy"
	"github.com/TecharoHQ/anubis/lib/policy/checker"
	"github.com/TecharoHQ/anubis/lib/policy/config"
	"github.com/gaissmai/bart"
)

var (
	ErrNoRemoteAddresses = errors.New("remoteaddress: no remote addresses defined")
)

func init() {}

type Factory struct{}

func (Factory) ValidateConfig(inp json.RawMessage) error {
	var fc fileConfig
	if err := json.Unmarshal([]byte(inp), &fc); err != nil {
		return fmt.Errorf("%w: %w", config.ErrUnparseableConfig, err)
	}

	if err := fc.Valid(); err != nil {
		return err
	}

	return nil
}

func (Factory) Create(inp json.RawMessage) (checker.Impl, error) {
	c := struct {
		RemoteAddr []netip.Prefix `json:"remote_addresses,omitempty" yaml:"remote_addresses,omitempty"`
	}{}

	if err := json.Unmarshal([]byte(inp), &c); err != nil {
		return nil, fmt.Errorf("%w: %w", config.ErrUnparseableConfig, err)
	}

	table := new(bart.Lite)

	for _, cidr := range c.RemoteAddr {
		table.Insert(cidr)
	}

	return &RemoteAddrChecker{
		prefixTable: table,
		hash:        internal.FastHash(string(inp)),
	}, nil
}

type fileConfig struct {
	RemoteAddr []string `json:"remote_addresses,omitempty" yaml:"remote_addresses,omitempty"`
}

func (fc fileConfig) Valid() error {
	var errs []error

	if len(fc.RemoteAddr) == 0 {
		errs = append(errs, ErrNoRemoteAddresses)
	}

	for _, cidr := range fc.RemoteAddr {
		if _, err := netip.ParsePrefix(cidr); err != nil {
			errs = append(errs, fmt.Errorf("%w: cidr %q is invalid: %w", config.ErrInvalidCIDR, cidr, err))
		}
	}

	if len(errs) != 0 {
		return fmt.Errorf("%w: %w", policy.ErrMisconfiguration, errors.Join(errs...))
	}

	return nil
}

type RemoteAddrChecker struct {
	prefixTable *bart.Lite
	hash        string
}

func (rac *RemoteAddrChecker) Check(r *http.Request) (bool, error) {
	host := r.Header.Get("X-Real-Ip")
	if host == "" {
		return false, fmt.Errorf("%w: header X-Real-Ip is not set", policy.ErrMisconfiguration)
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false, fmt.Errorf("%w: %s is not an IP address: %w", policy.ErrMisconfiguration, host, err)
	}

	return rac.prefixTable.Contains(addr), nil
}

func (rac *RemoteAddrChecker) Hash() string {
	return rac.hash
}
