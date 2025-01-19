# Lumera DevNet Test Scripts

## Prerequisites

- **Docker**: Ensure Docker is installed and up to date.  

      sudo apt-get update
      sudo apt-get install docker-ce docker-ce-cli containerd.io docker-compose-plugin

  
  If you still see old references to `docker-compose`, update to the latest Docker & Docker Compose plugin.

## Building the DevNet

Use the `make` target to build the DevNet with a custom genesis file:

    # For example:

    make make devnet-build EXTERNAL_CLAIMS_FILE=/home/desktop/Documents/lumera/claims.csv EXTERNAL_GENESIS_FILE=/home/desktop/Documents/lumera/tests/e2e/supernode/test_genesis_supernode.json

    make devnet-up

    # For clean up

    make devnet-down

    make devnet-clean



## Genesis Configuration

The DevNet uses a custom genesis file with the following key parameters (feel free to update them): 

- **Slashing**:
  - Signed blocks window: 10 blocks
  - Downtime jail duration: 60 seconds
- **Staking**:
  - Unbonding time: 60 seconds
  - Bond denomination: `ulumen`
- **Supernode**:
  - Minimum stake requirement: 834,637,515,648 ulumen
  - Default power reduction: 824,637,515,648 ulumen 

---

## Available Scripts

1. **supernode.sh**  
   Tests supernode lifecycle operations:
   - Registration
   - Start/Stop operations
   - Status updates
   - Deregistration  
   Run **after** the DevNet is operational.

2. **jailing.sh**  
   Tests validator jailing and unjailing mechanisms.
   - **Requires**:
     - `setup_five_supernodes.sh`
     - Active validator setup
     - (Optional) Changing the default validator number
     - Make sure has enough stake to withstand slashing
    - We increase the unbonding time (like 600s) to a value greater than jail time (eg: 60s) to make sure that the validator is not fully removed     while being jailed, as it will add an extra step of making a validator validator request again (which we dont have support yet, requires to a have submit json setting file with validator request, will add this in futre though in devnet docker file) in both scenarios, the same hooks is triggered `AfterValidatorBonded` so fulfills the purpose.

3. **delegation.sh**  
   Tests self-delegation changes and their effects on supernode status.
   - **Requires**:
     - `setup_five_supernodes.sh`
     - Active validator setup
     - (Optional) Changing the default validator number

4. **Helper Scripts**  
   - **setup_five_supernodes.sh**  
     Sets up 5 supernodes on the network.
     - Prerequisite for `jailing.sh` and `delegation.sh`
     - Can be run after the DevNet is operational

---

## Usage

1. **Start the DevNet** (using `docker compose up` or your chosen method).
2. **Run `setup_five_supernodes.sh`** to configure five supernodes.
3. **Run other test scripts** as needed, providing validator numbers where required:
   
       ./jailing.sh    
       ./delegation.sh 

---

## Available Commands

Below are the commands for interacting with the Supernode module .

### Query Commands

    # Get module parameters
    lumerad query supernode params

    # Get supernode information
    lumerad query supernode get-super-node [validator-address]

    # List all supernodes
    lumerad query supernode list-super-nodes

    # Get top supernodes for a specific block
    lumerad query supernode get-top-super-nodes-for-block [block-height]

### Transaction Commands

    # Register a new supernode
    lumerad tx supernode register-supernode [validator-address] [ip-address] [version] [supernode-account]

    # Deregister a supernode
    lumerad tx supernode deregister-supernode [validator-address]

    # Start a supernode
    lumerad tx supernode start-supernode [validator-address]

    # Stop a supernode
    lumerad tx supernode stop-supernode [validator-address] [reason]

    # Update supernode information
    lumerad tx supernode update-supernode [validator-address] [ip-address] [version] [supernode-account]


