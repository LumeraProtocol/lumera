import { ethers } from "hardhat";

async function main() {
  const [deployer] = await ethers.getSigners();
  console.log("Deploying contracts with account:", deployer.address);
  console.log(
    "Account balance:",
    ethers.formatEther(await ethers.provider.getBalance(deployer.address)),
    "LUME"
  );

  // --- ActionClient ---
  console.log("\n--- Deploying ActionClient ---");
  const ActionClient = await ethers.getContractFactory("ActionClient");
  const actionClient = await ActionClient.deploy();
  await actionClient.waitForDeployment();
  const actionAddr = await actionClient.getAddress();
  console.log("ActionClient deployed to:", actionAddr);

  // --- SupernodeClient ---
  console.log("\n--- Deploying SupernodeClient ---");
  const SupernodeClient = await ethers.getContractFactory("SupernodeClient");
  const supernodeClient = await SupernodeClient.deploy();
  await supernodeClient.waitForDeployment();
  const snAddr = await supernodeClient.getAddress();
  console.log("SupernodeClient deployed to:", snAddr);

  // --- LumeraDashboard ---
  console.log("\n--- Deploying LumeraDashboard ---");
  const Dashboard = await ethers.getContractFactory("LumeraDashboard");
  const dashboard = await Dashboard.deploy();
  await dashboard.waitForDeployment();
  const dashAddr = await dashboard.getAddress();
  console.log("LumeraDashboard deployed to:", dashAddr);

  // --- Summary ---
  console.log("\n========================================");
  console.log("Deployment complete!");
  console.log("========================================");
  console.log("ActionClient:    ", actionAddr);
  console.log("SupernodeClient: ", snAddr);
  console.log("LumeraDashboard: ", dashAddr);
  console.log("\nRun interaction script:");
  console.log(
    `  ACTION_CLIENT=${actionAddr} SUPERNODE_CLIENT=${snAddr} DASHBOARD=${dashAddr} npx hardhat run scripts/interact.ts --network devnet`
  );
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
