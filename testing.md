
1- Get the address for a key (this is base address)

pasteld keys show validator1_key --keyring-backend test

- address: pastel14hval0ywmumjnaa2c7efaxu6expctvwcx4p9zx
  name: validator1_key





2- Get the validator address
 pasteld keys show validator1_key --keyring-backend test --bech val

address: pastelvaloper14hval0ywmumjnaa2c7efaxu6expctvwc6rx73j
  name: validator1_key




1. To check available wallet balance:

pasteld query bank balances pastel1rwm8m4peuwslycwmmzl2uqsr7fdzfffh5a9xuz


2. To check your delegated (staked) amount:
```bash
docker exec pastel-validator1 pasteld query staking delegations pastel1rwm8m4peuwslycwmmzl2uqsr7fdzfffh5a9xuz
```

3. To check your validator's total delegations:
```bash
docker exec pastel-validator1 pasteld query staking validator pastelvaloper1rwm8m4peuwslycwmmzl2uqsr7fdzfffhgtza0k
```

4. To check any unclaimed rewards:
```bash
docker exec pastel-validator1 pasteld query distribution rewards pastel1rwm8m4peuwslycwmmzl2uqsr7fdzfffh5a9xuz
```

Try these commands to get a complete picture of your tokens. Based on your earlier configuration, you should see:
- Some available balance in your wallet
- 900000000000000 upsl staked on your validator-  


pasteld tx supernode register-supernode \
  pastelvaloper14hval0ywmumjnaa2c7efaxu6expctvwc6rx73j \
  192.168.1.1 \
  1.0 \
  pastel14hval0ywmumjnaa2c7efaxu6expctvwcx4p9zx \
  --from validator1_key \
  --keyring-backend test \
  --chain-id pastel-devnet-1


query the trnsaction with tx id from the above command to see the results 

pasteld query tx <tx-id>



pasteld query supernode list-super-nodes
Use "pasteld query supernode [command] --help" for more information about a command.
root@31bfd495e636:~# pasteld query supernode list-super-nodes
pagination:
  total: "1"
supernodes:
- metrics: {}
  prev_ip_addresses:
  - address: 192.168.1.1
    height: "183"
  states:
  - height: "183"
    state: SUPERNODE_STATE_ACTIVE
  supernode_account: pastel14hval0ywmumjnaa2c7efaxu6expctvwcx4p9zx
  validator_address: pastelvaloper14hval0ywmumjnaa2c7efaxu6expctvwc6rx73j
  version: "1.0"
root@31bfd495e636:~# pasteld query supernode list-super-nodes


pasteld tx supernode stop-supernode \
 pastelvaloper14hval0ywmumjnaa2c7efaxu6expctvwc6rx73j \
 "Maintenance" \
 --from validator1_key \
 --keyring-backend test \
 --chain-id pastel-devnet-1

try stopping again, should give an error



start now

pasteld tx supernode start-supernode \
 pastelvaloper14hval0ywmumjnaa2c7efaxu6expctvwc6rx73j \
 --from validator1_key \
 --keyring-backend test \
 --chain-id pastel-devnet-1


root@31bfd495e636:~# pasteld query supernode list-super-nodes
pagination:
  total: "1"
supernodes:
- metrics: {}
  prev_ip_addresses:
  - address: 192.168.1.1
    height: "183"
  states:
  - height: "183"
    state: SUPERNODE_STATE_ACTIVE
  - height: "1455"
    state: SUPERNODE_STATE_STOPPED
  - height: "1494"
    state: SUPERNODE_STATE_ACTIVE
  supernode_account: pastel14hval0ywmumjnaa2c7efaxu6expctvwcx4p9zx
  validator_address: pastelvaloper14hval0ywmumjnaa2c7efaxu6expctvwc6rx73j
  version: "1.0"


Update testing

pasteld tx supernode update-supernode \
  pastelvaloper14hval0ywmumjnaa2c7efaxu6expctvwc6rx73j \
  "192.168.1.2" \
  "2.0" \
  pastel14hval0ywmumjnaa2c7efaxu6expctvwcx4p9zx \
 --from validator1_key \
 --keyring-backend test \
 --chain-id pastel-devnet-1



pasteld tx supernode deregister-supernode \
  pastelvaloper14hval0ywmumjnaa2c7efaxu6expctvwc6rx73j \
 --from validator1_key \
 --keyring-backend test \
 --chain-id pastel-devnet-1

 oot@31bfd495e636:~# pasteld query supernode list-super-nodes
pagination:
  total: "1"
supernodes:
- metrics: {}
  prev_ip_addresses:
  - address: 192.168.1.1
    height: "183"
  - address: 192.168.1.2
    height: "1896"
  states:
  - height: "183"
    state: SUPERNODE_STATE_ACTIVE
  - height: "1455"
    state: SUPERNODE_STATE_STOPPED
  - height: "1494"
    state: SUPERNODE_STATE_ACTIVE
  - height: "2008"
    state: SUPERNODE_STATE_DISABLED
  supernode_account: pastel14hval0ywmumjnaa2c7efaxu6expctvwcx4p9zx
  validator_address: pastelvaloper14hval0ywmumjnaa2c7efaxu6expctvwc6rx73j
  version: "2.0"








# Validator 1
echo "=== Validator 1 ==="
docker exec -it pastel-validator1 pasteld keys show validator1_key --keyring-backend test
docker exec -it pastel-validator1 pasteld keys show validator1_key --keyring-backend test --bech val

# Validator 2
echo -e "\n=== Validator 2 ==="
docker exec -it pastel-validator2 pasteld keys show validator2_key --keyring-backend test
docker exec -it pastel-validator2 pasteld keys show validator2_key --keyring-backend test --bech val

# Validator 3
echo -e "\n=== Validator 3 ==="
docker exec -it pastel-validator3 pasteld keys show validator3_key --keyring-backend test
docker exec -it pastel-validator3 pasteld keys show validator3_key --keyring-backend test --bech val

# Validator 4
echo -e "\n=== Validator 4 ==="
docker exec -it pastel-validator4 pasteld keys show validator4_key --keyring-backend test --bech valel-validator5 pasteld keys show validator5_key --keyring-bac




=== Validator 1 ===
- address: pastel1j4qv6akqk6ale4j93x3xcw9gkerf9vqe8v082y
  name: validator1_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"A/pVLZ/tya8j1JEBm/EFfdkzrl/M/gPO5RvODkdpfZ01"}'
  type: local

- address: pastelvaloper1j4qv6akqk6ale4j93x3xcw9gkerf9vqem6gues
  name: validator1_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"A/pVLZ/tya8j1JEBm/EFfdkzrl/M/gPO5RvODkdpfZ01"}'
  type: local


=== Validator 2 ===
- address: pastel10hmadwl68dwntcg30l9fawcq6v6tzgs20gwm33
  name: validator2_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"A4AjMli7W9zZc/TvyexQO0dbTu1tnl6YpcFVwNyUlQj2"}'
  type: local

- address: pastelvaloper10hmadwl68dwntcg30l9fawcq6v6tzgs2n7fqz9
  name: validator2_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"A4AjMli7W9zZc/TvyexQO0dbTu1tnl6YpcFVwNyUlQj2"}'
  type: local


=== Validator 3 ===
- address: pastel1g4mdtzexwt7fepqzjjf6juhxxej7jh02996ckj
  name: validator3_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"AnvJJ0VKjaiH0QUgqu7INJlym3mq3MHcGO6TbvrDg5Vz"}'
  type: local

- address: pastelvaloper1g4mdtzexwt7fepqzjjf6juhxxej7jh02enar9x
  name: validator3_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"AnvJJ0VKjaiH0QUgqu7INJlym3mq3MHcGO6TbvrDg5Vz"}'
  type: local


=== Validator 4 ===
- address: pastel1e59yppcj5sw4xkkqgqyhfs2n9e7d73shdaxsck
  name: validator4_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"AiaOfiZmZtpv4ag86INtJrGoNf0m9oI0+DNk5hiFVC+j"}'
  type: local

- address: pastelvaloper1e59yppcj5sw4xkkqgqyhfs2n9e7d73sh3tpttz
  name: validator4_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"AiaOfiZmZtpv4ag86INtJrGoNf0m9oI0+DNk5hiFVC+j"}'
  type: local


=== Validator 5 ===
- address: pastel1gj69t79s6uaj02d0unpg0wwmeq62rw39k56d27
  name: validator5_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"AyHW5nIYXLeBZfXevDG87OGwoE1GeExlYHwZby8Kl9IF"}'
  type: local

- address: pastelvaloper1gj69t79s6uaj02d0unpg0wwmeq62rw392zake2
  name: validator5_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"AyHW5nIYXLeBZfXevDG87OGwoE1GeExlYHwZby8Kl9IF"}'
  type: local



# Register Validator 1
docker exec -it pastel-validator1 pasteld tx supernode register-supernode \
  pastelvaloper1j4qv6akqk6ale4j93x3xcw9gkerf9vqem6gues \
  192.168.1.1 \
  1.0 \
  pastel1j4qv6akqk6ale4j93x3xcw9gkerf9vqe8v082y \
  --from validator1_key \
  --keyring-backend test \
  --chain-id pastel-devnet-1

# Register Validator 2
docker exec -it pastel-validator2 pasteld tx supernode register-supernode \
  pastelvaloper10hmadwl68dwntcg30l9fawcq6v6tzgs2n7fqz9 \
  192.168.1.2 \
  1.0 \
  pastel10hmadwl68dwntcg30l9fawcq6v6tzgs20gwm33 \
  --from validator2_key \
  --keyring-backend test \
  --chain-id pastel-devnet-1

# Register Validator 3
docker exec -it pastel-validator3 pasteld tx supernode register-supernode \
  pastelvaloper1g4mdtzexwt7fepqzjjf6juhxxej7jh02enar9x \
  192.168.1.3 \
  1.0 \
  pastel1g4mdtzexwt7fepqzjjf6juhxxej7jh02996ckj \
  --from validator3_key \
  --keyring-backend test \
  --chain-id pastel-devnet-1

# Register Validator 4
docker exec -it pastel-validator4 pasteld tx supernode register-supernode \
  pastelvaloper1e59yppcj5sw4xkkqgqyhfs2n9e7d73sh3tpttz \
  192.168.1.4 \
  1.0 \
  pastel1e59yppcj5sw4xkkqgqyhfs2n9e7d73shdaxsck \
  --from validator4_key \
  --keyring-backend test \
  --chain-id pastel-devnet-1

# Register Validator 5
docker exec -it pastel-validator5 pasteld tx supernode register-supernode \
  pastelvaloper1gj69t79s6uaj02d0unpg0wwmeq62rw392zake2 \
  192.168.1.5 \
  1.0 \
  pastel1gj69t79s6uaj02d0unpg0wwmeq62rw39k56d27 \
  --from validator5_key \
  --keyring-backend test \
  --chain-id pastel-devnet-1



#  Now Jailing a validator



desktop@desktop-OMEN-by-HP-Laptop-15-ce0xx:~$ docker exec -it pastel-validator5 pasteld keys show validator5_key --keyring-backend test
docker exec -it pastel-validator5 pasteld keys show validator5_key --keyring-backend test --bech val
- address: pastel1h7gjmwktr7xkgj4znrxd866p7uckgv58n6wtv0
  name: validator5_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"AoHiGhxLgP2VzXnV2IROJVRrm9026XjUjA0d8wZDEqNe"}'
  type: local

- address: pastelvaloper1h7gjmwktr7xkgj4znrxd866p7uckgv580vfslm
  name: validator5_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"AoHiGhxLgP2VzXnV2IROJVRrm9026XjUjA0d8wZDEqNe"}'
  type: local









Stop a validator container (e.g., validator2)
docker stop pastel-validator2

You can monitor the validator's status with:
docker exec -it pastel-validator1 pasteld query staking validator \
  pastelvaloper10hmadwl68dwntcg30l9fawcq6v6tzgs2n7fqz9

After the validator is jailed, you can restart the container
docker start pastel-validator2



- address: pastel1h7gjmwktr7xkgj4znrxd866p7uckgv58n6wtv0
  name: validator5_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"AoHiGhxLgP2VzXnV2IROJVRrm9026XjUjA0d8wZDEqNe"}'
  type: local

- address: pastelvaloper1h7gjmwktr7xkgj4znrxd866p7uckgv580vfslm
  name: validator5_key
  pubkey: '{"@type":"/cosmos.crypto.secp256k1.PubKey","key":"AoHiGhxLgP2VzXnV2IROJVRrm9026XjUjA0d8wZDEqNe"}'
  type: local

pasteld tx supernode register-supernode \
  pastelvaloper1h7gjmwktr7xkgj4znrxd866p7uckgv580vfslm \
  192.168.1.1 \
  1.0 \
  pastel1h7gjmwktr7xkgj4znrxd866p7uckgv58n6wtv0 \
  --from validator5_key \
  --keyring-backend test \
  --chain-id pastel-devnet-1


desktop@desktop-OMEN-by-HP-Laptop-15-ce0xx:~$ docker exec -it pastel-validator1 pasteld query staking validator pastelvaloper1h7gjmwktr7xkgj4znrxd866p7uckgv580vfslm
validator:
  commission:
    commission_rates:
      max_change_rate: "10000000000000000"
      max_rate: "200000000000000000"
      rate: "100000000000000000"
    update_time: "2025-01-03T04:05:35.804432936Z"
  consensus_pubkey:
    type: /cosmos.crypto.ed25519.PubKey
    value: sn85nyQxr5PzQsSFj0uQfTUA4uo4pvw+Tz2PdtJgKlo=
  delegator_shares: "900000000000000000000000000000000"
  description:
    moniker: validator5
  jailed: true
  min_self_delegation: "1"
  operator_address: pastelvaloper1h7gjmwktr7xkgj4znrxd866p7uckgv580vfslm
  status: BOND_STATUS_UNBONDING
  tokens: "891000000000000"
  unbonding_height: "1457"
  unbonding_ids:
  - "1"
  unbonding_time: "2025-01-24T09:04:39.631167365Z"

desktop@desktop-OMEN-by-HP-Laptop-15-ce0xx:~$ docker exec pastel-validator1 pasteld q tx E0B102AA7D23F5B02BA992DF569CD6F837593538A92A7947A7B4A222DDBA00E7
code: 18
codespace: sdk
data: ""
events:
- attributes:
  - index: true
    key: fee
    value: ""
  - index: true
    key: fee_payer
    value: pastel1h7gjmwktr7xkgj4znrxd866p7uckgv58n6wtv0
  type: tx
- attributes:
  - index: true
    key: acc_seq
    value: pastel1h7gjmwktr7xkgj4znrxd866p7uckgv58n6wtv0/1
  type: tx
- attributes:
  - index: true
    key: signature
    value: fagcEbSVJn3ZmLVpucq7bMBO68GQw3kJYtBhtGIqpdJr6nEWeVtcpwrENrhziW8014gRRRYjlKexyodVLKQ27g==
  type: tx
gas_used: "38149"
gas_wanted: "200000"
height: "1659"
info: ""
logs: []
raw_log: 'failed to execute message; message index: 0: validator pastelvaloper1h7gjmwktr7xkgj4znrxd866p7uckgv580vfslm
  is jailed and cannot register a supernode: invalid request'
timestamp: "2025-01-03T09:22:40Z"
tx:
  '@type': /cosmos.tx.v1beta1.Tx
  auth_info:
    fee:
      amount: []
      gas_limit: "200000"
      granter: ""
      payer: ""
    signer_infos:
    - mode_info:
        single:
          mode: SIGN_MODE_DIRECT
      public_key:
        '@type': /cosmos.crypto.secp256k1.PubKey
        key: AoHiGhxLgP2VzXnV2IROJVRrm9026XjUjA0d8wZDEqNe
      sequence: "1"
    tip: null
  body:
    extension_options: []
    memo: ""
    messages:
    - '@type': /pastel.supernode.MsgRegisterSupernode
      creator: pastel1h7gjmwktr7xkgj4znrxd866p7uckgv58n6wtv0
      ipAddress: 192.168.1.1
      supernodeAccount: pastel1h7gjmwktr7xkgj4znrxd866p7uckgv58n6wtv0
      validatorAddress: pastelvaloper1h7gjmwktr7xkgj4znrxd866p7uckgv580vfslm
      version: "1.0"
    non_critical_extension_options: []
    timeout_height: "0"
  signatures:
  - fagcEbSVJn3ZmLVpucq7bMBO68GQw3kJYtBhtGIqpdJr6nEWeVtcpwrENrhziW8014gRRRYjlKexyodVLKQ27g==
txhash: E0B102AA7D23F5B02BA992DF569CD6F837593538A92A7947A7B4A222DDBA00E7




docker exec pastel-validator5 pasteld tx slashing unjail --from validator5_key --keyring-backend test --chain-id pastel-devnet-1 --yes

loper1h7gjmwktr7xkgj4znrxd866p7uckgv580vfslm
validator:
  commission:
    commission_rates:
      max_change_rate: "10000000000000000"
      max_rate: "200000000000000000"
      rate: "100000000000000000"
    update_time: "2025-01-03T04:05:35.804432936Z"
  consensus_pubkey:
    type: /cosmos.crypto.ed25519.PubKey
    value: sn85nyQxr5PzQsSFj0uQfTUA4uo4pvw+Tz2PdtJgKlo=
  delegator_shares: "900000000000000000000000000000000"
  description:
    moniker: validator5
  min_self_delegation: "1"
  operator_address: pastelvaloper1h7gjmwktr7xkgj4znrxd866p7uckgv580vfslm
  status: BOND_STATUS_BONDED
  tokens: "891000000000000"
  unbonding_height: "1457"
  unbonding_ids:
  - "1"
  unbonding_time: "2025-01-24T09:04:39.631167365Z"


desktop@desktop-OMEN-by-HP-Laptop-15-ce0xx:~$ docker exec pastel-validator5 pasteld tx supernode register-supernode pastelvaloper1h7gjmwktr7xkgj4znrxd866p7uckgv580vfslm 192.168.1.1 1.0 pastel1h7gjmwktr7xkgj4znrxd866p7uckgv58n6wtv0 --from validator5_key --keyring-backend test --chain-id pastel-devnet-1 --yes
code: 0
codespace: ""
data: ""
events: []
gas_used: "0"
gas_wanted: "0"
height: "0"
info: ""
logs: []
raw_log: ""
timestamp: ""
tx: null
txhash: 6EA2E43630C61CC1EFA8284FECBC4E42F11EE2DCB7667E8086D8480E620D6AEF

desktop@desktop-OMEN-by-HP-Laptop-15-ce0xx:~$ docker exec pastel-validator5 pasteld query supernode list-super-nodes
pagination:
  total: "5"
supernodes:
- metrics: {}
  prev_ip_addresses:
  - address: 192.168.1.2
    height: "6"
  states:
  - height: "6"
    state: SUPERNODE_STATE_ACTIVE
  supernode_account: pastel1tpxjg6yqeey6v2rldxhcf9q6dvs5y6vyhdknua
  validator_address: pastelvaloper1tpxjg6yqeey6v2rldxhcf9q6dvs5y6vytm3g0f
  version: "1.0"
- metrics: {}
  prev_ip_addresses:
  - address: 192.168.1.4
    height: "10"
  states:
  - height: "10"
    state: SUPERNODE_STATE_ACTIVE
  supernode_account: pastel10xs7qzf7xuxkt5p8tndn6k2a994nt9qj3q3ft4
  validator_address: pastelvaloper10xs7qzf7xuxkt5p8tndn6k2a994nt9qjdkkjcp
  version: "1.0"
- metrics: {}
  prev_ip_addresses:
  - address: 192.168.1.1
    height: "4"
  states:
  - height: "4"
    state: SUPERNODE_STATE_ACTIVE
  supernode_account: pastel13x9rczq9r8s2tpqfm6q7lxes49ck5k8j8pkesx
  validator_address: pastelvaloper13x9rczq9r8s2tpqfm6q7lxes49ck5k8jmh3zrj
  version: "1.0"
- metrics: {}
  prev_ip_addresses:
  - address: 192.168.1.1
    height: "1758"
  states:
  - height: "1758"
    state: SUPERNODE_STATE_ACTIVE
  supernode_account: pastel1h7gjmwktr7xkgj4znrxd866p7uckgv58n6wtv0
  validator_address: pastelvaloper1h7gjmwktr7xkgj4znrxd866p7uckgv580vfslm
  version: "1.0"
- metrics: {}
  prev_ip_addresses:
  - address: 192.168.1.3
    height: "8"
  states:
  - height: "8"
    state: SUPERNODE_STATE_ACTIVE
  supernode_account: pastel1m2fewfe6rlrphe0xuzkfuwa6cyn496znwc4pqu
  validator_address: pastelvaloper1m2fewfe6rlrphe0xuzkfuwa6cyn496znjwj6ng
  version: "1.0"