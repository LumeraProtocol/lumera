import { ethers } from "hardhat";

// Contract addresses — set via env vars or paste after deployment.
const ACTION_CLIENT = process.env.ACTION_CLIENT || "";
const SUPERNODE_CLIENT = process.env.SUPERNODE_CLIENT || "";
const DASHBOARD = process.env.DASHBOARD || "";

// Precompile addresses (can also be called directly without deploying contracts)
const ACTION_PRECOMPILE = "0x0000000000000000000000000000000000000901";
const SUPERNODE_PRECOMPILE = "0x0000000000000000000000000000000000000902";

async function main() {
  const [signer] = await ethers.getSigners();
  console.log("Interacting as:", signer.address);
  console.log(
    "Balance:",
    ethers.formatEther(await ethers.provider.getBalance(signer.address)),
    "LUME\n"
  );

  // -----------------------------------------------------------------------
  // 1) Direct precompile calls (no deployment needed)
  // -----------------------------------------------------------------------
  console.log("=== Direct Precompile Calls ===\n");

  await directActionQueries();
  await directSupernodeQueries();

  // -----------------------------------------------------------------------
  // 2) Calls via deployed contracts (if addresses provided)
  // -----------------------------------------------------------------------
  if (ACTION_CLIENT) {
    console.log("\n=== ActionClient Contract ===\n");
    await actionClientQueries(ACTION_CLIENT);
  }

  if (SUPERNODE_CLIENT) {
    console.log("\n=== SupernodeClient Contract ===\n");
    await supernodeClientQueries(SUPERNODE_CLIENT);
  }

  if (DASHBOARD) {
    console.log("\n=== LumeraDashboard Contract ===\n");
    await dashboardQueries(DASHBOARD);
  }

  if (!ACTION_CLIENT && !SUPERNODE_CLIENT && !DASHBOARD) {
    console.log(
      "\nTip: Deploy contracts first with `npm run deploy:devnet`, then",
      "pass addresses as env vars to see contract-mediated queries."
    );
  }
}

// ---------------------------------------------------------------------------
// Direct precompile interactions
// ---------------------------------------------------------------------------

async function directActionQueries() {
  const abi = [
    "function getParams() view returns (uint256, uint256, uint64, uint64, int64, string, string)",
    "function getActionFee(uint64) view returns (uint256, uint256, uint256)",
  ];
  const action = new ethers.Contract(ACTION_PRECOMPILE, abi, ethers.provider);

  // getParams
  const params = await action.getParams();
  console.log("Action Module Params (direct):");
  console.log("  baseActionFee:      ", ethers.formatUnits(params[0], 6), "LUME");
  console.log("  feePerKbyte:        ", ethers.formatUnits(params[1], 6), "LUME");
  console.log("  maxActionsPerBlock: ", params[2].toString());
  console.log("  minSuperNodes:      ", params[3].toString());
  console.log("  expirationDuration: ", params[4].toString(), "seconds");
  console.log("  superNodeFeeShare:  ", params[5]);
  console.log("  foundationFeeShare: ", params[6]);

  // getActionFee for 100 KB
  const fee = await action.getActionFee(100);
  console.log("\nFee estimate for 100 KB (direct):");
  console.log("  baseFee:  ", ethers.formatUnits(fee[0], 6), "LUME");
  console.log("  perKbFee: ", ethers.formatUnits(fee[1], 6), "LUME");
  console.log("  totalFee: ", ethers.formatUnits(fee[2], 6), "LUME");
}

async function directSupernodeQueries() {
  const abi = [
    "function getParams() view returns (uint256, uint64, uint64, string, uint64, uint64, uint64)",
    "function listSuperNodes(uint64, uint64) view returns (tuple(string, string, uint8, int64, string, string, string, uint64)[], uint64)",
  ];
  const supernode = new ethers.Contract(
    SUPERNODE_PRECOMPILE,
    abi,
    ethers.provider
  );

  // getParams
  const params = await supernode.getParams();
  console.log("\nSupernode Module Params (direct):");
  console.log("  minimumStake:        ", ethers.formatUnits(params[0], 6), "LUME");
  console.log("  reportingThreshold:  ", params[1].toString(), "blocks");
  console.log("  slashingThreshold:   ", params[2].toString(), "missed");
  console.log("  minSupernodeVersion: ", params[3]);
  console.log("  minCpuCores:         ", params[4].toString());
  console.log("  minMemGb:            ", params[5].toString(), "GB");
  console.log("  minStorageGb:        ", params[6].toString(), "GB");

  // listSuperNodes
  const [nodes, total] = await supernode.listSuperNodes(0, 5);
  console.log(`\nSupernodes (${total.toString()} total, showing first ${nodes.length}):`);
  for (const n of nodes) {
    const stateNames: Record<number, string> = {
      1: "Active",
      2: "Disabled",
      3: "Stopped",
      4: "Penalized",
      5: "Postponed",
    };
    console.log(`  ${n[0]} — ${stateNames[Number(n[2])] || "Unknown"} — ${n[4]}:${n[5]}`);
  }
}

// ---------------------------------------------------------------------------
// Contract-mediated interactions
// ---------------------------------------------------------------------------

async function actionClientQueries(addr: string) {
  const ActionClient = await ethers.getContractFactory("ActionClient");
  const client = ActionClient.attach(addr);

  // estimateFee
  const [baseFee, perKbFee, totalFee] = await client.estimateFee(100);
  console.log("Fee for 100 KB via ActionClient:");
  console.log("  baseFee: ", ethers.formatUnits(baseFee, 6), "LUME");
  console.log("  perKbFee:", ethers.formatUnits(perKbFee, 6), "LUME");
  console.log("  totalFee:", ethers.formatUnits(totalFee, 6), "LUME");

  // getModuleParams
  const params = await client.getModuleParams();
  console.log("Module params via contract:", params[5], "/", params[6], "fee split");
}

async function supernodeClientQueries(addr: string) {
  const SupernodeClient = await ethers.getContractFactory("SupernodeClient");
  const client = SupernodeClient.attach(addr);

  // totalSupernodeCount
  const count = await client.totalSupernodeCount();
  console.log("Total supernodes:", count.toString());

  // listNodes
  const [nodes, total] = await client.listNodes(0, 5);
  console.log(`Listed ${nodes.length} of ${total.toString()} nodes`);

  // getModuleParams
  const params = await client.getModuleParams();
  console.log("Min stake:", ethers.formatUnits(params[0], 6), "LUME");
  console.log("Min version:", params[3]);
}

async function dashboardQueries(addr: string) {
  const Dashboard = await ethers.getContractFactory("LumeraDashboard");
  const dashboard = Dashboard.attach(addr);

  // getNetworkOverview — single call combining both modules
  const overview = await dashboard.getNetworkOverview();
  console.log("Network Overview (single eth_call):");
  console.log("  Action base fee:    ", ethers.formatUnits(overview.baseActionFee, 6), "LUME");
  console.log("  Action fee/KB:      ", ethers.formatUnits(overview.feePerKbyte, 6), "LUME");
  console.log("  Max actions/block:  ", overview.maxActionsPerBlock.toString());
  console.log("  Min supernodes/action:", overview.minSuperNodes.toString());
  console.log("  SN min stake:       ", ethers.formatUnits(overview.minimumStake, 6), "LUME");
  console.log("  Total supernodes:   ", overview.totalSupernodes.toString());
  console.log("  Min SN version:     ", overview.minSupernodeVersion);
  console.log("  Min CPU/Mem/Disk:   ", overview.minCpuCores.toString(), "cores /",
    overview.minMemGb.toString(), "GB /", overview.minStorageGb.toString(), "GB");

  // isNetworkReady
  const [ready, totalSn, minReq] = await dashboard.isNetworkReady();
  console.log(`\nNetwork ready: ${ready} (${totalSn}/${minReq} supernodes)`);

  // estimateFeeWithContext for 500 KB
  const est = await dashboard.estimateFeeWithContext(500);
  console.log(`\nFee estimate for 500 KB:`);
  console.log("  Total fee:", ethers.formatUnits(est.totalFee, 6), "LUME");
  console.log("  Available supernodes:", est.availableSupernodes.toString());
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
