# Cascade Everlight - Funding Model & Calculations

**Doc ID:** CE-FM-001
**Status:** Draft
**Scope:** Operational sizing of the Everlight pool: per-byte payout rate, per-period top-up requirements, principal needed for endowment self-sufficiency, sensitivity analysis, and the manual-funding runbook for Phase 1.
**Companions:** Cascade-Everlight-Brief.md, Cascade-Everlight-Feature-Proposal.md, Cascade-Everlight-phase1-plan.md
**Helper scripts:** `scripts/everlight-budget.py`, `scripts/everlight-sn-reward.py`, `scripts/everlight-endowment.py`

---

## 1. Purpose and structure

The Feature Proposal defines the mechanism. This doc puts numbers on it.

The core Everlight idea is simple: **registration fees are one-time action revenue; Everlight pool payouts are extra recurring storage-retention revenue paid to SuperNodes after registration.** Every payment period, the pool pays eligible SNs in proportion to the Cascade data they continue to hold. This creates an ongoing compensation rail for retained storage instead of asking operators to absorb storage costs forever after a single registration/finalization fee.

The doc is split into two parts:

- **Part 1 (sections 2-8)** covers Phase 1 manual pool funding. It shows how large the extra monthly SN retention payout pool should be, what per-byte storage economics that implies, and how the Foundation tops up the pool until protocol-native funding grows.
- **Part 2 (sections 9-14)** covers Phase 3 endowment self-sufficiency. It shows how big a permanently-staked principal has to be so the extra recurring SN payouts can run from staking yield, ending the Foundation's ongoing top-up obligation. It also explains how the endowment behaves when LUME price moves.

Part 1 is needed to ship the upgrade. Part 2 is needed to plan toward exiting Foundation funding. Both share the same input set in section 2.

---

# Part 1 — Phase 1: Manual Pool Funding

## 2. Inputs (tune these)

The model is anchored to per-byte hardware cost and a target storage APR, instead of a per-SN dollar target. This makes it explicit that operators receive **ongoing payments beyond registration fees** in proportion to the storage they actually provide, with the protocol acting like a yield product on top of the operator's hardware investment.

| # | Variable | Symbol | Placeholder | Notes                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
|---|---|---|---|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 1 | Hardware cost per GiB per month, USD | `HW_rate` | $0.04/GiB/month | Hetzner reference: AX42 + extra 2TB drive = $81/month for ~2048 GiB usable -> $0.0395/GiB/month, rounded to $0.04. Per-byte hardware cost stays roughly constant or drops slightly across larger Hetzner tiers (AX52 / AX102 / AX162), so this single number works for the whole fleet at first approximation.                                                                                                                                                 |
| 2 | Target storage APR (annualized return paid TO operators on top of hardware cost) | `storage_APR` | 20% | The protocol pays operators their hardware cost plus this APR on top. 20% APR means a 2 TB SN earns ~$97/month vs ~$81 hardware = ~$16 margin (~20% over cost). Cross-tier check: per-byte payout = `HW_rate * (1 + storage_APR)` is uniform across storage tiers, so whales earn proportionally more in absolute terms but the same percentage margin against the chain-wide reference. Tune higher to attract operators, lower to compress the program cost. |
| 3 | LUME spot price in USD | `P_lume` | $0.30 | Update at sizing review. Used only to convert USD targets to LUME.                                                                                                                                                                                                                                                                                                                                                                                             |
| 4 | Active eligible SuperNode count | `N_sn` | 30 | Current fleet is 28 active SNs; using 30 as the planning number. Steady-state expectation, after STORAGE_FULL gating and `min_cascade_bytes_for_payment` floor.                                                                                                                                                                                                                                                                                                |
| 5 | Average per-SN cascade_kademlia_db_bytes | `B_sn` | 3 TiB | **Target fleet average, not current devnet reality.** Network design targets BIG storage: regular SNs at 2 TB+, whales at 10 TB+. With a mix of regulars and whales, ~3 TiB is the steady-state planning number. Early devnet / early mainnet will be much lower (10-200 GiB per SN) until ingest accumulates.                                                                                                                                                 |
| 6 | `payment_period_blocks` | `T_period` | 432000 | ~30 days at 6s blocks. Governance param, default in `params.go:85`.                                                                                                                                                                                                                                                                                                                                                                                            |
| 7 | Periods per year | `K_year` | ~12.17 | `T_year_blocks / T_period`. With 6s blocks: 5,256,000 / 432,000. Calendar-month math elsewhere in the doc rounds this to 12.                                                                                                                                                                                                                                                                                                                                  |
| 8 | LUME staking APR (validator yield) | `staking_APR` | 10% | **Different from storage_APR.** Used in Part 2 only, for sizing endowment principal. This is the yield earned BY the staked principal, not the yield paid TO operators.                                                                                                                                                                                                                                                                                        |
| 9 | Registration fee inflow per period | `F_reg_period` | TBD | `2% * (sum of action fees in last T_period blocks)`. Currently dependent on adoption; treat as 0 at devnet launch.                                                                                                                                                                                                                                                                                                                                             |
| 10 | Community Pool transfer cadence | `F_cp_period` | 0 | Governance-driven; episodic, not steady-state.                                                                                                                                                                                                                                                                                                                                                                                                                 |

#### Hardware cost reference (per Hetzner pricing)

| Storage tier | Reference machine | Approx monthly cost | Cost per GiB |
|---|---|---|---|
| Regular SN (2 TB) | AX42 + 2TB extra drive | ~$81 ($64 + $17) | ~$0.040/GiB/month |
| Heavy SN (5 TB) | AX52 + extra drives | ~$150 (estimate) | ~$0.030/GiB/month |
| Whale SN (10 TB) | AX102 or equivalent | ~$200-250 (estimate) | ~$0.020-0.025/GiB/month |
| Mega-whale (20 TB+) | AX162 / multi-disk | ~$300-400+ (estimate) | ~$0.015-0.020/GiB/month |

Per-byte cost actually *drops* as storage grows because fixed costs (CPU, RAM, base chassis) get amortized across more disks. Picking $0.04/GiB/month as the chain-wide reference uses the most expensive tier (regular 2 TB), which is conservative: it slightly over-pays whales relative to their actual hardware cost. That's intentional for Phase 1 to give whales extra incentive to bring big storage online.

---

## 3. Distribution mechanics

The Everlight pool is a module account that accumulates LUME from various funding sources (Foundation transfers, registration fee share, Community Pool transfers, eventually endowment yield) and pays it out to eligible SuperNodes on a fixed cadence. These payouts are **additional retention compensation** layered on top of existing action registration/finalization economics. This section covers the cadence and the all-or-nothing payout behavior, which are the two facts that drive every funding number in the rest of the doc.

### 3.1 Cadence: every `payment_period_blocks` blocks

Distribution is triggered automatically by the supernode keeper's `EndBlocker`, not by any external transaction or off-chain script. On every block the keeper compares the current block height to the last recorded distribution height. When the difference reaches `payment_period_blocks` (governance parameter, default `432000`), a distribution runs at the end of that block.

At Lumera's ~6-second block time, the default works out to:

```
100800 blocks * 6 s =   604,800 s =  7 days
432000 blocks * 6 s = 2,592,000 s = 30 days
```

So at default settings, **one distribution per month**. The cadence is governance-tunable via supernode `MsgUpdateParams`. Faster cadence (smaller `payment_period_blocks`) means more frequent but smaller payouts; slower cadence means lumpier ones. The monthly default was chosen as a compromise between operator pay-frequency expectations and the per-block overhead of running per-SN bank sends.

Throughout this document, "period" and "month" are used interchangeably. If the chain ever changes `payment_period_blocks`, all "monthly" numbers below scale accordingly.

### 3.2 Payout: the entire pool balance, every period

At each period boundary the keeper:

1. Reads the current pool balance, whatever has been transferred in since the last distribution, from any source.
2. Builds the eligible-SN list. An SN qualifies if its state is `ACTIVE` or `STORAGE_FULL`, its latest audit-epoch metrics report is fresh (within `metrics_freshness_max_blocks`), and its smoothed `cascade_kademlia_db_bytes` is at least `min_cascade_bytes_for_payment`.
3. Computes each eligible SN's `effectiveWeight = smoothedBytes * rampWeight`, where smoothing applies an exponential moving average over `measurement_smoothing_periods` and ramp-up is partial weight for new SNs over `new_sn_ramp_up_periods`.
4. Pays each SN `pool_balance * (effectiveWeight / total_effectiveWeight)`, truncated to integer `ulume`.
5. Records the new distribution height and emits per-SN and summary distribution events.

**The pool empties at every boundary.** There is no "X% of pool per period" cap, no per-byte minimum payment, no rate limiter, no reserve fund inside the pool. If 100,000 LUME is in the pool at the period boundary and there are eligible SNs, 100,000 LUME (minus tiny truncation dust, on the order of one `ulume` per SN per period) leaves the pool to those SNs in that block. The pool starts the next period at zero plus whatever inflows arrive during that period.

This is a deliberate design choice. Treating the pool as a strict pass-through makes the funding-sizing decision *the* decision: whatever you fund the pool with this week determines what SNs are paid this week. There is no smoothing or buffering at the pool level; smoothing happens only at the per-SN weight level *within* a period (via the EMA), and that affects who gets which slice of the pie, not how big the pie is.

### 3.3 Edge cases

These are the cases where the period boundary fires but no payout actually occurs. None of them break anything; they just advance the schedule and leave one of the two sides empty:

- **Pool balance is zero at the boundary.** Distribution emits a `pool_balance_zero` skip event, advances `LastDistributionHeight`, and exits. SNs receive no payout this period; the next period starts a fresh cycle.
- **No eligible SNs at the boundary.** Emit a `no_eligible_supernodes` skip event, advance height, exit. The pool balance carries over to the next period. This is the only case where the pool does *not* fully drain.
- **First period after upgrade.** The `LastDistributionHeight = 0` case is special-cased to avoid distributing on the genesis block; the first real distribution happens once `payment_period_blocks` have elapsed since the upgrade.

### 3.4 What this means for funding

The all-pool-at-once behavior is the central operational fact behind the rest of this document:

- Whatever LUME enters the pool by the period boundary leaves the pool at that boundary.
- To pay SNs `X` LUME this period, the pool must contain `X` LUME at period's end.
- Funding cadence and amount can be irregular intra-period (transfer once, transfer multiple times, top up mid-month, etc.); only the cumulative balance at the boundary matters.
- Forgetting a top-up is a missed period of pay, not a deferred payment. Operators should expect monthly payouts and the funder should expect to top up monthly.

The rest of Part 1 is the math for choosing the right top-up amount.

---

## 4. Target monthly pool budget

### 4.1 In LUME (aggregate pool budget)

The funding policy targets a flat per-byte retention payout. Multiply total eligible fleet bytes by the target per-byte storage payout and convert to LUME per period. At the default `payment_period_blocks = 432000` (~30 days), one period ≈ one calendar month, so per-period equals per-month:

```
per_byte_payout_monthly_usd = HW_rate * (1 + storage_APR)
pool_monthly_usd            = N_sn * B_sn_gib * per_byte_payout_monthly_usd
pool_period_usd             = pool_monthly_usd                    # 1 period ≈ 1 month at default
pool_period_lume            = pool_period_usd / P_lume
```

If `payment_period_blocks` is governance-tuned to a different cadence, scale per-period by `(T_period_seconds / seconds_per_month)`.

`B_sn_gib` is `B_sn` expressed in GiB (3 TiB = 3072 GiB).

With placeholder inputs (`HW_rate` = $0.04/GiB/month, `storage_APR` = 20%, `N_sn` = 30, `B_sn` = 3 TiB, `P_lume` = $0.30):

```
per_byte_payout_monthly = 0.04 * 1.20             = $0.048/GiB/month
pool_monthly_usd        = 30 * 3072 * 0.048        = $4,423.68/month
pool_period_usd         = pool_monthly_usd         = ~$4,423.68/period
pool_period_lume        = 4,423.68 / 0.30          = ~14,746 LUME/period
```

So at placeholder settings, the pool needs **~14.7k LUME paid out per month** in aggregate as extra recurring SN retention compensation, totaling **~177k LUME per year**, equivalent to ~$4,424/month or ~$53k/year for the entire fleet of 30 SNs holding 3 TiB each (90 TiB total).

These numbers come from `scripts/everlight-budget.py`; rerun it whenever the inputs change.

### 4.2 Required pool inflow per period

Pool inflow per period must equal pool outflow (the pool drains every period, see section 3.2):

```
Inflow_period = pool_period_lume
              = F_foundation_period + F_reg_period + F_cp_period + F_endowment_period
```

In Phase 1 with no significant registration-fee inflow or endowment yield:

```
F_foundation_period_required = pool_period_lume - F_reg_period - F_cp_period
                             ~= 14,746 LUME per period
```

This is the monthly Foundation top-up size at placeholder inputs.

### 4.3 What each SN actually receives

Registration/finalization fees are paid once. Everlight payouts repeat every payment period and are allocated by retained Cascade bytes. In the target funding model, the per-byte retention payout rate is uniform across the fleet, so a steady-state per-SN payout is just bytes times that rate:

```
payout_i_monthly_usd = bytes_i_gib * per_byte_payout_monthly_usd
                     = bytes_i_gib * HW_rate * (1 + storage_APR)
```

Equivalently, in terms of the average:

```
payout_i = pool_balance * (bytes_i / total_bytes)
         = average_payout * (bytes_i / average_bytes)
```

Use `scripts/everlight-sn-reward.py --sn-storage-gib X` to compute the reward for any given SN size.

#### Worked example (30 SNs, ~14.7k LUME/month retention pool, 20% storage_APR, $0.04/GiB/month HW reference)

Per-byte retention payout: `$0.048/GiB/month`, paid on top of the normal one-time action economics. Hardware cost per GiB drops as the SN gets bigger; the per-byte payout stays the same; the per-byte margin therefore *grows* with capacity. That matches the design intent: whales hold more bytes and are rewarded with the highest absolute margin.

| SN profile | Stored bytes | Per-month USD payout | Per-month LUME payout | Approx hardware cost | Margin (USD) | Margin (% over cost) |
|---|---|---|---|---|---|---|
| Regular SN (2 TB target) | 2 TiB = 2048 GiB | ~$98 | ~328 LUME | ~$81 | ~$17 | ~21% |
| Average SN (3 TB) | 3 TiB = 3072 GiB | ~$147 | ~491 LUME | ~$120 (~AX52 estimate) | ~$27 | ~22% |
| Heavy SN (5 TB) | 5 TiB = 5120 GiB | ~$246 | ~819 LUME | ~$150 | ~$96 | ~64% |
| Whale SN (10 TB target) | 10 TiB = 10240 GiB | ~$492 | ~1,638 LUME | ~$200-250 | ~$240-290 | ~95-145% |
| Mega-whale (20 TB+) | 20 TiB = 20480 GiB | ~$983 | ~3,277 LUME | ~$300-400 | ~$580-680 | ~145-225% |
| Below floor | 500 MiB | excluded (under 1 GiB) | 0 | n/a | n/a | n/a |

The margin percentages grow with size because per-byte hardware cost falls with bigger machines, while the protocol pays a flat per-byte rate. That is the explicit incentive for whales to bring big storage online: the more bytes they hold, the better their unit economics.

The chain-wide reference `HW_rate = $0.04` is anchored to the *most expensive* tier (regular 2 TB at $0.0395/GiB). If the typical operator runs cheaper hardware, all margins above are conservative. In practice, as the fleet matures, `HW_rate` can be re-tuned downward to compress whale margins while still keeping regulars profitable.

#### When does this matter

- **Storage parity.** If the fleet is roughly homogeneous (every SN at approximately 3 TiB), per-SN payouts are close to the average and all operators clear roughly the placeholder margin.
- **Storage variance (expected).** Whales naturally capture a larger payout share, which is the protocol's incentive for them to bring more storage online. Their margin in absolute USD is the largest because per-byte hardware cost drops at scale.
- **New SN ramp-up.** A new SN's `rampWeight` is below 1 for `new_sn_ramp_up_periods` periods. Its actual payout is reduced by `rampWeight`, with the displaced share spread across all other eligible SNs.
- **Below-floor SNs.** SNs below `min_cascade_bytes_for_payment` (default 1 GiB) are excluded entirely. Their would-be share goes to the eligible set, raising the average payout for those who qualify.

---

## 5. Phase 1 funding sources

In Phase 1 the pool is fed almost entirely by the Foundation, with two minor protocol-native streams that grow over time:

| Source | Expected share at devnet launch | Driver |
|---|---|---|
| Foundation direct transfers | ~95-100% | Manual `MsgSend` per the runbook in section 7. |
| Registration fee share (2%) | 0-5% | Grows with adoption. At devnet launch, ~0. |
| Community Pool transfers | episodic | One-off governance proposals. |

Phase 1 sustainability depends entirely on Foundation willingness. The point of Part 2 is to get out of this regime by sizing an endowment that replaces the Foundation top-up.

Importantly, the 2% registration fee share is only a contribution into the recurring retention pool. It is not the same thing as the one-time finalization payout to SNs. Everlight's reason to exist is that retained storage creates ongoing operator cost after registration, so SNs need a recurring payment stream beyond the original action fee.

### 5.1 The Foundation's funding model in plain terms

The Foundation's mechanical role in Phase 1 is identical to what `x/endowment` will do automatically in Phase 3: **the Foundation parks principal in its own wallet, claims rewards monthly, sends them to the pool**. Section 7 describes the runbook in detail.

This means Phase 1 is effectively a "manual Phase 3" with the Foundation acting as a temporary endowment. Once `x/endowment` ships, it replaces the *source* of principal (per-registration endowment fees paid by users, instead of Foundation grant) but keeps the mechanism identical. The Foundation can then recover its principal and the program continues to run on yield generated by user-paid endowments.

This framing matters for sizing: section 10 (Part 2) computes the principal needed for self-sufficiency. That same principal can be supplied either by a Foundation grant during Phase 1 or by `x/endowment` accumulation during Phase 3. The numbers are the same in both cases.

The net implication: there is no functional reason to wait for `x/endowment` to start running yield-funded payouts. The Foundation can begin the yield-funded model on day one of Phase 1 just by staking the appropriate principal and forwarding the rewards to the pool.

---

## 6. Phase 1 sensitivity

### 6.1 Pool budget vs. storage_APR and SN count

Holding `HW_rate = $0.04`, `B_sn = 3 TiB`, `P_lume = $0.30`. The bolded column (30) is the current fleet size.

| `storage_APR` \ `N_sn` | 10 | 20 | **30** | 50 | 100 |
|---|---|---|---|---|---|
| 0% (covers HW only, zero margin) | 4,096 LUME/mo | 8,192 | **12,288** | 20,480 | 40,960 |
| 10% (regular ~$10 margin) | 4,506 | 9,011 | **13,517** | 22,528 | 45,056 |
| 20% (placeholder, regular ~$17 margin) | 4,915 | 9,830 | **14,746** | 24,576 | 49,152 |
| 30% (regular ~$24 margin) | 5,325 | 10,650 | **15,974** | 26,624 | 53,248 |
| 50% (regular ~$40 margin) | 6,144 | 12,288 | **18,432** | 30,720 | 61,440 |
| 100% (regular ~$80 margin) | 8,192 | 16,384 | **24,576** | 40,960 | 81,920 |

### 6.2 Pool budget vs. fleet storage

Holding `HW_rate = $0.04`, `storage_APR = 20%`, `P_lume = $0.30`. As the fleet's average storage grows (more operator commitment, more Cascade adoption), the aggregate pool grows proportionally.

#### At N_sn = 30 (current fleet size)

| `B_sn` (avg per SN) | Total fleet bytes | LUME / month | USD / month | LUME / year |
|---|---|---|---|---|
| 1 TiB | 30 TiB | ~4,915 | ~$1,475 | ~58,982 |
| 2 TiB | 60 TiB | ~9,830 | ~$2,949 | ~117,965 |
| **3 TiB** (placeholder) | **90 TiB** | **~14,746** | **~$4,424** | **~176,947** |
| 5 TiB | 150 TiB | ~24,576 | ~$7,373 | ~294,912 |
| 10 TiB | 300 TiB | ~49,152 | ~$14,746 | ~589,824 |

#### At N_sn = 100 (mainnet growth target)

| `B_sn` (avg per SN) | Total fleet bytes | LUME / month | USD / month | LUME / year |
|---|---|---|---|---|
| 1 TiB | 100 TiB | ~16,384 | ~$4,915 | ~196,608 |
| 2 TiB | 200 TiB | ~32,768 | ~$9,830 | ~393,216 |
| 3 TiB | 300 TiB | ~49,152 | ~$14,746 | ~589,824 |
| 5 TiB | 500 TiB | ~81,920 | ~$24,576 | ~983,040 |
| 10 TiB | 1 PiB | ~163,840 | ~$49,152 | ~1,966,080 |

A 100-SN fleet at 3 TiB average is ~3.3x the 30-SN placeholder budget; same fleet at 10 TiB average is ~11x. Plan principal accordingly if Cascade adoption accelerates.

### 6.3 Sensitivity to LUME price

The formulas use `P_lume` only to convert USD targets to LUME. If LUME doubles, all LUME requirements halve (more LUME-denominated funding power per dollar). Foundation top-up budgets and endowment principal are therefore best set in USD-equivalent terms, with periodic re-pegging.

The asymmetry between this Phase 1 elasticity (Foundation can adjust its monthly LUME transfer freely) and the Phase 3 endowment elasticity (yield is fixed in LUME, USD purchasing power floats) is discussed in section 12.

---

## 7. Phase 1 manual-funding runbook

### 7.1 One-time setup (pre-upgrade)

1. Foundation identifies a permanent staking principal in its own wallet, sized per Part 2 (section 10).
2. Foundation delegates that principal across one or more validators per `MsgDelegate`. Diversify to bound slashing exposure.
3. Foundation records the supernode module account address (deterministic, derivable from module name) in its operator runbook. This is the Everlight pool address until Phase 3 splits it out.

### 7.2 Monthly top-up (cron)

Runs once per `T_period`, ideally a few hours before the period boundary so funds land in the pool before distribution:

1. Withdraw delegator rewards: `lumerad tx distribution withdraw-all-rewards --from foundation`
2. Compute current target inflow per section 4.1, accounting for any registration-fee share already in the pool. Use `scripts/everlight-budget.py` to recompute when inputs drift.
3. Send the difference to the pool: `lumerad tx bank send foundation <pool_addr> <amount>ulume`.
4. Verify pool balance via `Query.PoolState` matches expectation.

A simple shell script wrapping these three commands plus a `cron` entry suffices for Phase 1. Sample script: TODO add to `scripts/everlight-topup.sh`.

### 7.3 Adjustment triggers

Recompute and adjust the monthly target whenever any of these change materially:

- LUME price moves more than ~30% from the last sizing.
- `N_sn` (eligible) changes by more than ~25% (new operators onboarding, or attrition).
- Average `B_sn` changes by more than ~50% (operator adoption pattern shifts).
- Hetzner reference pricing moves materially (recompute `HW_rate`).
- Registration fee inflow becomes material (a non-trivial `F_reg_period`); reduce Foundation top-up by that amount.

If budget pressure exceeds Foundation tolerance, options are:
- Lower `storage_APR` via governance transparency (announce, then reduce).
- Raise `registration_fee_share_bps` via `MsgUpdateParams`.
- Initiate Community Pool transfers via governance.
- Accelerate Phase 3 endowment work.

### 7.4 Failure modes

- **Foundation misses a top-up.** Pool distributes whatever is in it (potentially zero). SNs see a missed payout. No protocol failure, but operator confidence erodes. Mitigation: cron script + alerting on pool balance vs. expected.
- **Foundation top-up arrives after period boundary.** Funds carry to the next period (no penalty, just delayed). Schedule top-up at least one block before boundary; even better, earlier in the period.
- **Validator slashing on Foundation principal.** Yield reduces but is not zero; principal slowly erodes. Diversify across validators, monitor, top up principal if needed.

---

## 8. Devnet vs Mainnet sizing

Devnet launches with placeholder values that produce small absolute LUME flows. The goal is to exercise the distribution mechanism, not to compensate operators commensurate with real costs.

| Variable | Devnet | Mainnet (initial)                                           |
|---|---|-------------------------------------------------------------|
| `storage_APR` | symbolic (e.g., 1%) | 20% (start; tune from operator feedback)                    |
| `HW_rate` | symbolic (e.g., $0.001/GiB) | $0.04/GiB/month (Hetzner AX42 reference)                    |
| `N_sn` | 3-5 | 30 (current fleet); plan to handle 50-100 as adoption grows |
| `B_sn` (target) | n/a (small, however much actually accumulates) | 3 TiB average (regulars 2 TB+, whales 10 TB+)               |
| `payment_period_blocks` | 1000-5000 (faster cadence for visibility) | 432000 (~1 month)                                          |
| `P_lume` | n/a (set in genesis or test env) | $0.30 (current placeholder)                                 |
| Foundation principal | small fixed grant | sized per Part 2                                            |
| Endowment | not active | not active until Phase 3                                    |

For devnet, the operationally simplest thing is a single one-shot Foundation transfer of, e.g., 10k LUME, then observe distribution events for several periods. No cron needed at devnet scale.

---

# Part 2 — Phase 3: Endowment-Funded Self-Sufficiency

## 9. Phase progression and Part 2 trigger

Each phase shifts the funding burden away from the Foundation and toward protocol-native sources.

### Phase 1 (Part 1)

Foundation does the heavy lifting. Protocol contributes 0-5% via registration fee share. Pool budget computed in section 4.

### Phase 2 (LEP-6 audit hardening)

No new funding sources, but full payouts now require demonstrated retention via challenges. The *effective* payout per SN drops until the SN passes the audit gate, which naturally reduces inflow requirements during the ramp until the fleet is fully audited. Phase 2 does not change the budget formulas; it changes what fraction of eligible SNs actually qualify for full payout.

### Phase 3 (this part)

Endowment yield becomes the dominant source. The Foundation top-up tapers to supplemental.

| Source | Expected share at Phase 3 maturity | Driver |
|---|---|---|
| Endowment staking yield | 60-90% | Per-registration tiered fees, principal staked, yield to pool. Scales with Cascade adoption. |
| Registration fee share (2%) | 5-20% | Continues. |
| Foundation transfers | 0-20% | Tapers to supplemental / emergency only. |
| Community Pool transfers | episodic | Same as before. |

The crossover point (when endowment yield alone covers the per-period pool budget from Part 1) is the formal Phase 1 -> Phase 3 economic transition. Sizing the principal needed to reach that point is what the rest of Part 2 covers.

### What `x/endowment` actually buys

The mechanism is identical to what the Foundation can do manually (section 5.1). Phase 3 does NOT introduce a new payout flow; it introduces a new *source* of principal:

- **Manual path (Phase 1):** principal is a Foundation grant from treasury.
- **Endowment path (Phase 3):** principal accumulates from per-registration endowment fees paid by Cascade users.

That is the sustainability shift. Once user-paid endowments cover the principal, the Foundation can recover its temporary grant and the program self-funds. Mechanically the LUME flow into the pool is the same in both phases.

---

## 10. Endowment principal sizing

The principal `P_endow` that, when staked at `staking_APR`, generates exactly the annual outflow required by Part 1:

```
R_annual_lume      = pool_period_lume * K_year
P_endow_required   = R_annual_lume / staking_APR
```

Note: `staking_APR` here is the *validator yield* on bonded LUME (placeholder 10%), not the *storage APR* paid to operators (placeholder 20%). The two are unrelated and tuned separately.

With placeholders (carrying `pool_period_lume` ≈ 14,746 from section 4.1, `staking_APR` = 10%):

```
R_annual_lume      = 14,746 * 12      ~= 176,947 LUME / year
P_endow_required   = 176,947 / 0.10   ~= 1,769,472 LUME staked permanently
```

**To run the extra recurring SN retention payouts from staking yield alone with the placeholder inputs (30 SNs at 3 TiB avg, 20% storage_APR, $0.04 HW_rate, 10% staking_APR, $0.30 LUME), ~1.77M LUME has to be permanently bonded somewhere whose rewards reach the pool.**

That can be:
- **Phase 3 protocol path:** total endowment principal accumulated from per-registration endowment fees, delegated by `x/endowment` across whitelisted validators, yield routed to pool.
- **Pre-Phase 3 manual path:** Foundation parks principal in its own wallet, claims rewards monthly, sends to pool. Functionally identical, just operationally manual.

These numbers come from `scripts/everlight-endowment.py`; rerun it whenever the inputs change.

### Risk buffer for slashing

`risk_buffer_bps` (Phase 3 param) skims a fraction of yield to recapitalize against slashing events, so the principal target should be slightly larger than the bare formula:

```
P_endow_required_with_buffer = P_endow_required / (1 - risk_buffer_bps / 10000)
```

For `risk_buffer_bps = 500` (5%): buffer-adjusted principal is ~1.86M LUME instead of ~1.77M.

---

## 11. Endowment sensitivity

Principal scales linearly with annual outflow and inversely with staking APR. Below: principal needed at 10% staking APR, for various combinations of `storage_APR`, `B_sn`, and `N_sn`. The bolded row is the current placeholder (30 SNs, 3 TiB avg, 20% storage_APR, P_lume = $0.30).

| storage_APR | N_sn | B_sn | Annual LUME | Principal at 10% staking_APR | Principal at 5% | Principal at 15% |
|---|---|---|---|---|---|---|
| 10% | 30 | 3 TiB | ~162,201 | ~1.62M | ~3.24M | ~1.08M |
| 20% | 30 | 1 TiB | ~58,982 | ~590K | ~1.18M | ~393K |
| **20%** | **30** | **3 TiB** | **~176,947** | **~1.77M** | **~3.54M** | **~1.18M** |
| 20% | 30 | 5 TiB | ~294,912 | ~2.95M | ~5.90M | ~1.97M |
| 20% | 30 | 10 TiB | ~589,824 | ~5.90M | ~11.80M | ~3.93M |
| 20% | 50 | 3 TiB | ~294,912 | ~2.95M | ~5.90M | ~1.97M |
| 20% | 100 | 3 TiB | ~589,824 | ~5.90M | ~11.80M | ~3.93M |
| 50% | 30 | 3 TiB | ~221,184 | ~2.21M | ~4.42M | ~1.47M |
| 100% | 30 | 3 TiB | ~294,912 | ~2.95M | ~5.90M | ~1.97M |

**Sensitivity to LUME price.** Same as Part 1: principal targets are LUME-denominated and scale inversely with LUME price moves. Set the principal in USD-equivalent terms, re-peg on price moves. See section 12 for the more nuanced post-Phase-3 dynamics.

**Sensitivity to staking_APR.** Halving validator yield doubles the required principal. A change in Lumera's inflation parameters or validator-set composition therefore directly resizes the endowment ask.

**Sensitivity to fleet growth.** Doubling N_sn doubles the principal. Doubling B_sn (avg storage per SN) doubles the principal too. These are linear and additive.

---

## 12. How endowment adapts to LUME price changes

The endowment principal is denominated in LUME and so is the staking yield it produces. The pool *target* defined in Part 1 is denominated in USD (via `HW_rate * (1 + storage_APR) * total_bytes`). When LUME price moves, the two diverge.

### 12.1 The asymmetry

Endowment yield is fixed in LUME terms once the principal is committed:

```
yield_annual_lume = P_endow * staking_APR
```

That number does not change when LUME price moves. What moves is its USD purchasing power.

The pool target, by contrast, is anchored in USD:

```
pool_period_usd        = N_sn * B_sn_gib * HW_rate * (1 + storage_APR)   # 1 period ≈ 1 month at default
pool_period_lume_target = pool_period_usd / P_lume
```

That number scales inversely with `P_lume`. When LUME doubles, half as many LUME are needed to deliver the same USD target.

So:

- **LUME appreciates 2x:** endowment yield (LUME) is unchanged but now buys 2x the USD it used to. The pool over-funds the USD target by 2x. Operators receive a windfall in USD; their margin doubles.
- **LUME depreciates 2x:** endowment yield buys half the USD it used to. The pool under-funds by 2x. Operator USD margin halves; some regulars may fall below break-even.

The on-chain code does not auto-adjust. The supernode keeper's distribution logic just empties the pool every period, regardless of whether the LUME amount overshoots or undershoots the USD target.

### 12.2 Adaptation strategies

There are two practical responses, neither requires a new code path:

**(a) Accept floating USD margin.** Treat the program as paying a fixed per-byte LUME rate (whatever the endowment yield happens to produce per total fleet byte). Operator USD income floats with LUME price. Operators self-correct over time: appreciation attracts more operators (compressing per-SN payout in LUME terms), depreciation thins the fleet. Simple, low governance overhead.

**(b) Rebase via governance after material price moves.** Submit a `MsgUpdateParams` to change the off-chain `storage_APR` target and / or recompute `HW_rate` against the latest USD-equivalent operator costs. Then resize the LUME inflow into the pool to match: in Phase 3 this means redirecting some yield away from the pool (eg into a buffer) when the pool would over-fund, or topping up from elsewhere when it under-funds. The supernode keeper does NOT consume `storage_APR` or `HW_rate` directly today; those are off-chain sizing inputs in this funding doc. Rebasing thus translates to operational decisions about how much of the endowment yield to route to the pool versus retain.

### 12.3 Implication for sizing

When the endowment is sized for self-sufficiency at a given LUME price (`P_lume_at_sizing`), what it actually delivers in USD scales with `P_lume_current / P_lume_at_sizing`. Size principal at LUME = $0.30 and LUME drops to $0.15 → you get half the USD purchasing power per period until governance rebases or operators leave.

Two practical mitigations during Phase 3 design:

- **Size principal conservatively.** Pick a `P_lume_at_sizing` lower than current price (eg current price * 0.7) so a typical price drawdown leaves the program at parity rather than at deficit.
- **Keep Foundation as backstop.** Even after Phase 3 ships, keep a Foundation top-up channel ready for material LUME drawdowns. The endowment automates the steady state; the Foundation handles tail risk.

In Phase 1 (manual funding) this issue is much smaller: the Foundation cron script can be retuned monthly. In Phase 3 the rebase has to come from governance, which is slower.

### 12.4 Worked numerical example

Take the placeholder principal of ~1.77M LUME (sized at LUME = $0.30, see section 10):

- Annual yield = 1.77M * 0.10 = ~177k LUME / year
- At LUME = $0.30: yield value = ~$53,084 / year (matches the pool target exactly)
- At LUME = $0.60: yield value = ~$106,168 / year (target only needs ~$53k -> 2x over-fund)
- At LUME = $0.15: yield value = ~$26,542 / year (target needs ~$53k -> 2x under-fund)

The endowment is therefore *price-elastic on the upside* (gives operators a bonus when LUME appreciates) and *price-inelastic on the downside* (operators take the hit when LUME depreciates) unless governance steps in.

---

## 13. Open decisions

These are inputs to be resolved before mainnet launch. Devnet upgrade does not need them locked.

- **`storage_APR` for mainnet.** Pure business call. The placeholder 20% gives regulars ~21% margin over hardware cost; pick higher to attract operators more aggressively, lower to compress program cost. Worth surveying current operators on what margin keeps them in.
- **`HW_rate` calibration.** Current placeholder uses Hetzner regular 2TB tier ($0.04/GiB/month). If most operators run cheaper hardware (different provider, owned hardware, etc.), the chain-wide HW_rate could be tuned down without hurting operator economics.
- **Storage tier expectations.** If the fleet skews more toward whales (avg `B_sn` >> 3 TiB), aggregate budget grows linearly. Plan for that scenario explicitly.
- **`staking_APR` baseline.** Pull from current Lumera validator data once mainnet stabilizes. Update Section 10 numbers.
- **`P_lume_at_sizing` for endowment principal.** Pick a conservative value (eg current price * 0.7) to give the principal headroom against price drawdowns.
- **Initial Foundation principal commitment.** Section 10 gives the formula; the actual number is a Foundation budget decision.
- **Registration fee inflow projections.** Once Cascade adoption has even a few weeks of mainnet data, plug actual `F_reg_period` into section 4.2 and reduce Foundation transfers by that amount.
- **Endowment tier pricing (Phase 3).** Belongs in a dedicated Phase 3 design doc, but principal target from section 10 anchors that work.
- **Community Pool transfer cadence.** Coordinate with governance.
- **LUME-price-shock playbook.** Decide ahead of time whether Phase 3 will accept floating USD margin (12.2 option a) or rebase via governance (option b). Operators benefit from knowing which.

---

## 14. Maintenance

Re-check this doc at the start of each quarter, and after any of:
- A material LUME price move.
- A change to `payment_period_blocks` or `registration_fee_share_bps`.
- A material shift in `N_sn` or per-SN storage average (`B_sn`).
- A change in Hetzner reference pricing affecting `HW_rate`.
- A staking APR change affecting Part 2 sizing.
- A phase transition (Phase 2 / Phase 3 ship).

Update inputs in section 2; downstream sections will give the new numbers.

For ad-hoc what-if calculations between formal reviews, use the helper scripts:

- `scripts/everlight-budget.py` for aggregate pool budget.
- `scripts/everlight-sn-reward.py --sn-storage-gib X` for per-SN reward.
- `scripts/everlight-endowment.py` for endowment principal sizing.

All three accept the same input flags and produce numbers consistent with the placeholders and worked examples in this document.
