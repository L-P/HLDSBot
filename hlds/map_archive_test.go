package hlds_test

import (
	"hldsbot/hlds"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var testVaultExpectedFailures = map[string]error{
	// {{{ zip
	"vault_test/twhl-vault-386.zip":  hlds.MissingBSPErr,
	"vault_test/twhl-vault-1412.zip": hlds.MissingBSPErr,
	"vault_test/twhl-vault-3349.zip": hlds.MissingBSPErr,
	"vault_test/twhl-vault-4333.zip": hlds.MissingBSPErr,
	"vault_test/twhl-vault-4523.zip": hlds.MissingBSPErr,
	"vault_test/twhl-vault-4688.zip": hlds.MissingBSPErr,
	"vault_test/twhl-vault-5433.zip": hlds.MissingBSPErr,
	"vault_test/twhl-vault-5514.zip": hlds.MissingBSPErr,
	"vault_test/twhl-vault-5688.zip": hlds.MissingBSPErr,

	"vault_test/twhl-vault-1467.zip": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-5149.zip": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-5292.zip": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-5484.zip": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-5607.zip": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-5612.zip": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-6141.zip": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-6336.zip": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-6521.zip": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-6619.zip": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-6798.zip": hlds.MultipleBSPErr,

	// This one is peculiar. Will open correctly if parsed manually using the
	// zip package but not its fs.FS interface implementation.
	// There's a "C2 B4" in it that's not a valid unicode char.
	"vault_test/twhl-vault-6621.zip": hlds.InvalidPathErr,
	// }}}

	// {{{ 7z
	"vault_test/twhl-vault-1485.7z": hlds.MissingBSPErr,
	"vault_test/twhl-vault-2453.7z": hlds.MissingBSPErr,

	"vault_test/twhl-vault-2665.7z": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-3776.7z": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-5130.7z": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-5132.7z": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-5133.7z": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-5145.7z": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-5192.7z": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-5613.7z": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-5655.7z": hlds.MultipleBSPErr,
	"vault_test/twhl-vault-6733.7z": hlds.MultipleBSPErr,
	// }}}
}

func TestVaultArchives(t *testing.T) {
	for _, v := range []string{"zip", "7z"} {
		t.Run(v, func(t *testing.T) {
			testVaultArchives(t, v)
		})
	}
}

func testVaultArchives(t *testing.T, ext string) {
	if _, err := os.Stat("vault_test"); os.IsNotExist(err) {
		t.Skip("vault_test dir not present, cannot test its archives")
		return
	}

	paths, err := filepath.Glob("vault_test/*." + ext)
	require.NoError(t, err, "can get archives from test dir")
	require.NotEmpty(t, paths, "no file present in vault_test directory")

	for _, path := range paths {
		ma, err := hlds.ReadMapArchiveFromFile(path)
		if err != nil {
			if expected, ok := testVaultExpectedFailures[path]; ok {
				require.ErrorIs(t, err, expected)
				continue
			}
			require.NoError(t, err, "can parse %s without error: %s", ext, path)
		}

		require.NoError(t, ma.Close(), "can close %s without error %s", ext, path)
	}
}
