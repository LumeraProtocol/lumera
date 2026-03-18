import { expect } from "chai";
import { ethers } from "hardhat";

// These tests run against a live Lumera node (devnet or localnode).
// They are NOT unit tests — they require the precompiles to be available.
//
// Run with:
//   npx hardhat test --network devnet
//   npx hardhat test --network localnode

describe("Lumera Precompile Contracts", function () {
  // Increase timeout for on-chain calls
  this.timeout(60_000);

  describe("Direct precompile calls", function () {
    it("should query action module params via precompile", async function () {
      const abi = [
        "function getParams() view returns (uint256, uint256, uint64, uint64, int64, string, string)",
      ];
      const action = new ethers.Contract(
        "0x0000000000000000000000000000000000000901",
        abi,
        ethers.provider
      );

      const params = await action.getParams();
      expect(params[0]).to.be.gt(0n, "baseActionFee should be > 0");
      expect(params[2]).to.be.gt(0n, "maxActionsPerBlock should be > 0");
    });

    it("should query supernode module params via precompile", async function () {
      const abi = [
        "function getParams() view returns (uint256, uint64, uint64, string, uint64, uint64, uint64)",
      ];
      const supernode = new ethers.Contract(
        "0x0000000000000000000000000000000000000902",
        abi,
        ethers.provider
      );

      const params = await supernode.getParams();
      expect(params[0]).to.be.gt(0n, "minimumStake should be > 0");
      expect(params[3]).to.not.equal("", "minSupernodeVersion should be non-empty");
    });

    it("should calculate action fees via precompile", async function () {
      const abi = [
        "function getActionFee(uint64) view returns (uint256, uint256, uint256)",
      ];
      const action = new ethers.Contract(
        "0x0000000000000000000000000000000000000901",
        abi,
        ethers.provider
      );

      const [baseFee, perKbFee, totalFee] = await action.getActionFee(100);
      expect(baseFee).to.be.gt(0n);
      expect(totalFee).to.equal(baseFee + perKbFee * 100n);
    });
  });

  describe("ActionClient contract", function () {
    it("should deploy and estimate fees", async function () {
      const Factory = await ethers.getContractFactory("ActionClient");
      const client = await Factory.deploy();
      await client.waitForDeployment();

      const [baseFee, perKbFee, totalFee] = await client.estimateFee(200);
      expect(baseFee).to.be.gt(0n);
      expect(totalFee).to.equal(baseFee + perKbFee * 200n);
    });

    it("should read module params through contract", async function () {
      const Factory = await ethers.getContractFactory("ActionClient");
      const client = await Factory.deploy();
      await client.waitForDeployment();

      const params = await client.getModuleParams();
      // baseActionFee
      expect(params[0]).to.be.gt(0n);
      // maxActionsPerBlock
      expect(params[2]).to.be.gt(0n);
    });
  });

  describe("SupernodeClient contract", function () {
    it("should deploy and query total count", async function () {
      const Factory = await ethers.getContractFactory("SupernodeClient");
      const client = await Factory.deploy();
      await client.waitForDeployment();

      // total count may be 0 on fresh devnet, but should not revert
      const count = await client.totalSupernodeCount();
      expect(count).to.be.gte(0n);
    });

    it("should read module params through contract", async function () {
      const Factory = await ethers.getContractFactory("SupernodeClient");
      const client = await Factory.deploy();
      await client.waitForDeployment();

      const params = await client.getModuleParams();
      // minimumStake
      expect(params[0]).to.be.gt(0n);
      // minSupernodeVersion
      expect(params[3]).to.not.equal("");
    });

    it("should list nodes without revert", async function () {
      const Factory = await ethers.getContractFactory("SupernodeClient");
      const client = await Factory.deploy();
      await client.waitForDeployment();

      const [nodes, total] = await client.listNodes(0, 10);
      expect(total).to.be.gte(0n);
      expect(nodes.length).to.be.lte(10);
    });
  });

  describe("LumeraDashboard contract", function () {
    it("should return a complete network overview", async function () {
      const Factory = await ethers.getContractFactory("LumeraDashboard");
      const dashboard = await Factory.deploy();
      await dashboard.waitForDeployment();

      const overview = await dashboard.getNetworkOverview();
      // Action module data
      expect(overview.baseActionFee).to.be.gt(0n);
      expect(overview.maxActionsPerBlock).to.be.gt(0n);
      // Supernode module data
      expect(overview.minimumStake).to.be.gt(0n);
      expect(overview.minSupernodeVersion).to.not.equal("");
    });

    it("should estimate fees with context", async function () {
      const Factory = await ethers.getContractFactory("LumeraDashboard");
      const dashboard = await Factory.deploy();
      await dashboard.waitForDeployment();

      const est = await dashboard.estimateFeeWithContext(500);
      expect(est.dataSizeKbs).to.equal(500n);
      expect(est.totalFee).to.be.gt(0n);
      expect(est.baseFee).to.be.gt(0n);
    });

    it("should report network readiness", async function () {
      const Factory = await ethers.getContractFactory("LumeraDashboard");
      const dashboard = await Factory.deploy();
      await dashboard.waitForDeployment();

      const [ready, totalSn, minReq] = await dashboard.isNetworkReady();
      // On fresh devnet, may not be ready — just verify it doesn't revert
      // and returns consistent data
      if (ready) {
        expect(totalSn).to.be.gte(minReq);
      } else {
        expect(totalSn).to.be.lt(minReq);
      }
    });
  });
});
