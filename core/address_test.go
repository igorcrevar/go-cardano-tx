package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddressParts(t *testing.T) {
	wallet1, err := GenerateWallet(true)
	require.NoError(t, err)

	wallet3 := NewWallet(wallet1.SigningKey, nil)

	cliUtils := NewCliUtils(ResolveCardanoCliBinary(TestNetNetwork))

	wallet1KeyHash, err := GetKeyHash(wallet1.VerificationKey)
	require.NoError(t, err)

	wallet1StakeKeyHash, err := GetKeyHash(wallet1.StakeVerificationKey)
	require.NoError(t, err)

	walletAddress, walletStakeAddress, err := cliUtils.GetWalletAddress(
		wallet1.VerificationKey, wallet1.StakeVerificationKey, TestNetProtocolMagic)
	require.NoError(t, err)

	wallet3Address, _, err := cliUtils.GetWalletAddress(
		wallet3.VerificationKey, wallet3.StakeVerificationKey, 0)
	require.NoError(t, err)

	cWalletAddress, err := NewCardanoAddressFromString(walletAddress)
	require.NoError(t, err)

	assert.Equal(t, wallet1KeyHash, cWalletAddress.GetInfo().Payment.String())
	assert.Equal(t, wallet1StakeKeyHash, cWalletAddress.GetInfo().Stake.String())
	assert.False(t, cWalletAddress.GetInfo().Network.IsMainNet())
	assert.False(t, cWalletAddress.GetInfo().Payment.IsScript)
	assert.False(t, cWalletAddress.GetInfo().Stake.IsScript)

	assert.Equal(t, walletAddress, cWalletAddress.String())

	baseAddr, err := NewBaseAddress(TestNetNetwork, wallet1.VerificationKey, wallet1.StakeVerificationKey)
	require.NoError(t, err)

	rewardAddr, err := NewRewardAddress(TestNetNetwork, wallet1.StakeVerificationKey)
	require.NoError(t, err)

	enterpriseAddr, err := NewEnterpriseAddress(MainNetNetwork, wallet1.VerificationKey)
	require.NoError(t, err)

	assert.Equal(t, walletAddress, baseAddr.String())
	assert.Equal(t, wallet3Address, enterpriseAddr.String())
	assert.Equal(t, walletStakeAddress, rewardAddr.String())
}

func TestNewAddress(t *testing.T) {
	addresses := []string{
		"addr1qx2fxv2umyhttkxyxp8x0dlpdt3k6cwng5pxj3jhsydzer3n0d3vllmyqwsx5wktcd8cc3sq835lu7drv2xwl2wywfgse35a3x",
		"addr1z8phkx6acpnf78fuvxn0mkew3l0fd058hzquvz7w36x4gten0d3vllmyqwsx5wktcd8cc3sq835lu7drv2xwl2wywfgs9yc0hh",
		"addr1yx2fxv2umyhttkxyxp8x0dlpdt3k6cwng5pxj3jhsydzerkr0vd4msrxnuwnccdxlhdjar77j6lg0wypcc9uar5d2shs2z78ve",
		"addr1x8phkx6acpnf78fuvxn0mkew3l0fd058hzquvz7w36x4gt7r0vd4msrxnuwnccdxlhdjar77j6lg0wypcc9uar5d2shskhj42g",
		"addr1gx2fxv2umyhttkxyxp8x0dlpdt3k6cwng5pxj3jhsydzer5pnz75xxcrzqf96k",
		"addr128phkx6acpnf78fuvxn0mkew3l0fd058hzquvz7w36x4gtupnz75xxcrtw79hu",
		"addr1vx2fxv2umyhttkxyxp8x0dlpdt3k6cwng5pxj3jhsydzers66hrl8",
		"addr1w8phkx6acpnf78fuvxn0mkew3l0fd058hzquvz7w36x4gtcyjy7wx",
		"stake1uyehkck0lajq8gr28t9uxnuvgcqrc6070x3k9r8048z8y5gh6ffgw",
		"stake178phkx6acpnf78fuvxn0mkew3l0fd058hzquvz7w36x4gtcccycj5",

		"addr1q9d66zzs27kppmx8qc8h43q7m4hkxp5d39377lvxefvxd8j7eukjsdqc5c97t2zg5guqadepqqx6rc9m7wtnxy6tajjvk4a0kze4ljyuvvrpexg5up2sqxj33363v35gtew",

		"addr_test1qz2fxv2umyhttkxyxp8x0dlpdt3k6cwng5pxj3jhsydzer3n0d3vllmyqwsx5wktcd8cc3sq835lu7drv2xwl2wywfgs68faae",
		"addr_test1zrphkx6acpnf78fuvxn0mkew3l0fd058hzquvz7w36x4gten0d3vllmyqwsx5wktcd8cc3sq835lu7drv2xwl2wywfgsxj90mg",
		"addr_test1yz2fxv2umyhttkxyxp8x0dlpdt3k6cwng5pxj3jhsydzerkr0vd4msrxnuwnccdxlhdjar77j6lg0wypcc9uar5d2shsf5r8qx",
		"addr_test1xrphkx6acpnf78fuvxn0mkew3l0fd058hzquvz7w36x4gt7r0vd4msrxnuwnccdxlhdjar77j6lg0wypcc9uar5d2shs4p04xh",
		"addr_test1gz2fxv2umyhttkxyxp8x0dlpdt3k6cwng5pxj3jhsydzer5pnz75xxcrdw5vky",
		"addr_test12rphkx6acpnf78fuvxn0mkew3l0fd058hzquvz7w36x4gtupnz75xxcryqrvmw",
		"addr_test1vz2fxv2umyhttkxyxp8x0dlpdt3k6cwng5pxj3jhsydzerspjrlsz",
		"addr_test1wrphkx6acpnf78fuvxn0mkew3l0fd058hzquvz7w36x4gtcl6szpr",
		"stake_test1uqehkck0lajq8gr28t9uxnuvgcqrc6070x3k9r8048z8y5gssrtvn",
		"stake_test17rphkx6acpnf78fuvxn0mkew3l0fd058hzquvz7w36x4gtcljw6kf",
	}

	for i, a := range addresses {
		addr, err := NewCardanoAddressFromString(a)
		assert.NoError(t, err, "%s has error: %v", a, err)

		if err == nil {
			assert.Equal(t, i <= 10, addr.GetInfo().Network.IsMainNet(), "%s should be on mainnet: %v", a, i <= 9)

			switch i % 11 {
			case 0, 1, 2, 3:
				assert.Equal(t, BaseAddress, addr.GetInfo().AddressType)
			case 4, 5:
				assert.Equal(t, PointerAddress, addr.GetInfo().AddressType)
			case 6, 7:
				assert.Equal(t, EnterpriseAddress, addr.GetInfo().AddressType)
			case 8, 9:
				assert.Equal(t, RewardAddress, addr.GetInfo().AddressType)
			}

			assert.Equal(t, a, addr.String())

			newAddr, err := addr.GetInfo().ToCardanoAddress()
			assert.NoError(t, err)

			if err == nil {
				assert.Equal(t, a, newAddr.String())
			}
		}
	}
}

func TestByronAddress(t *testing.T) {
	addrs := []string{
		"Ae2tdPwUPEYwFx4dmJheyNPPYXtvHbJLeCaA96o6Y2iiUL18cAt7AizN2zG",
		"Ae2tdPwUPEZFRbyhz3cpfC2CumGzNkFBN2L42rcUc2yjQpEkxDbkPodpMAi",
		"37btjrVyb4KDXBNC4haBVPCrro8AQPHwvCMp3RFhhSVWwfFmZ6wwzSK6JK1hY6wHNmtrpTf1kdbva8TCneM2YsiXT7mrzT21EacHnPpz5YyUdj64na",
		"37btjrVyb4KEB2STADSsj3MYSAdj52X5FrFWpw2r7Wmj2GDzXjFRsHWuZqrw7zSkwopv8Ci3VWeg6bisU9dgJxW5hb2MZYeduNKbQJrqz3zVBsu9nT",
	}

	for i, addrStr := range addrs {
		addr, err := NewCardanoAddressFromString(addrStr)
		require.NoError(t, err)

		newAddr, err := addr.GetInfo().ToCardanoAddress()
		require.NoError(t, err)

		require.Equal(t, addr.String(), newAddr.String())
		require.Equal(t, addrStr, newAddr.String())
		require.Equal(t, ByronAddress, addr.GetInfo().AddressType)

		if i < 2 {
			require.Equal(t, MainNetNetwork, addr.GetInfo().Network)
		} else {
			require.Equal(t, TestNetNetwork, addr.GetInfo().Network)
		}
	}
}
