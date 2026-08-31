# 📣 Lumera EVM Migration — Community Announcements (🚨 PRELIMINARY)

**Status**: 🚨 **PRELIMINARY** — the exact **upgrade block height** and **migration-window close date** are still **TBD**. These posts set expectations early; a **final** announcement with concrete heights/dates will follow. Keep the PRELIMINARY banner on every post until the dates are locked.
**Audience**: Telegram + Discord (public users, validators, supernode operators)
**Format**: single Unicode-emoji version of each message — renders correctly on both Telegram and Discord.
**Before posting**: replace `TBD` once the upgrade block and window-close are decided, then drop the PRELIMINARY banner for the final versions. Links target the GitHub `v1.20.0` tag.

---

## 📌 Pinned one-liner

Pin this short version at the top of each channel:

```
🚨 PRELIMINARY 🚨 Lumera is going EVM-compatible! 🦊 Migrating your account will be FREE 💸 and stays open ~3 months ⏳ after the upgrade — your balance is always safe 💰, but staking, delegations & unbonding vanish forever 🌧️ if you miss it. Exact block/dates TBD. Guide 👉 https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/migration.md
```

---

## 1️⃣ General public

```
============================================================
🚨🚨🚨  THIS IS A PRELIMINARY ANNOUNCEMENT  🚨🚨🚨
============================================================
📣 Lumera Is Going EVM — The Upgrade WILL Affect Your Accounts ⚡

Lumera is upgrading to add Ethereum/EVM compatibility 🦊. Accounts move to Ethereum-style keys, so the SAME recovery phrase will derive a brand-new Lumera address after the upgrade. To carry your account onto the upgraded chain, you migrate it. 🔁

💸 It's free
Every migration transaction is completely fee-free — no gas, even though your new address starts empty.

✅ What migration does
Moves your balance 💰, staking 🥩, delegations, rewards 🎁, and unbonding ⏳ to your new EVM-compatible address — in one free transaction.

⚠️ Miss the window and…
Your liquid tokens are fine — you can still send them from the legacy account to an EVM account. 👍
But your delegations, staking, and unbonding? Gone. Unrecoverable. Lost in time, like tears in the rain. 🌧️
If you stake, migrating in time isn't optional.

⏳ You would have ~3 months after the upgrade block!!!
The migration window stays open for about 3 months after the upgrade block (closes at: TBD at this point). Don't leave it to the last block. 📆

🧩 Never sent a transaction from your wallet?
If your address holds a balance but has never sent anything, it has no public key on-chain yet. Do one small send first — even to yourself — before migrating. It records your key and makes migration painless. ✨

🔐 Multisig?
Multisig uses a dedicated migration script, not the normal wallet flow.

📖 Full step-by-step guide:
https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/migration.md

All those rewards… lost in time. Don't let them. Migrate. 🌧️
```

---

## 2️⃣ Validators & supernode operators

```
============================================================
🚨🚨🚨  THIS IS A PRELIMINARY ANNOUNCEMENT  🚨🚨🚨
============================================================
🛠️ Validators & Supernode Operators — EVM Migration ⚡

Your operator account also needs migrating to an EVM-compatible key 🔑 — but DON'T use the public Keplr/Portal flow. Use the helper scripts in the repo. They add pre-flight estimates 🔍, destination-freshness checks, post-migration verification ✅, and structured exit codes. Like all migrations, these transactions are FREE 💸 (no gas).

⚠️ Don't miss the window: liquid balance can be sent out later, but delegations, staking, and unbonding are lost permanently 🌧️ if you don't migrate in time. Exact upgrade block & window close: TBD.

📂 Scripts folder:
https://github.com/LumeraProtocol/lumera/tree/v1.20.0/scripts

Pick your script:
• 🛡️ Validators → migrate-validator.sh
  https://github.com/LumeraProtocol/lumera/blob/v1.20.0/scripts/migrate-validator.sh
• 🖥️ Single account (supernode / relayer / regular) → migrate-account.sh
  https://github.com/LumeraProtocol/lumera/blob/v1.20.0/scripts/migrate-account.sh
• 🔐 Multisig → migrate-multisig.sh
  https://github.com/LumeraProtocol/lumera/blob/v1.20.0/scripts/migrate-multisig.sh
• 📦 Bulk / many accounts → migrate-batch.sh (see migrate-batch.md)
  https://github.com/LumeraProtocol/lumera/blob/v1.20.0/scripts/migrate-batch.sh

Key reminders:
• 🔑 Your validator CONSENSUS key (priv_validator_key.json) is NOT touched — only the operator account key changes (secp256k1 → eth_secp256k1).
• 🩺 Run the pre-flight check before stopping your node, and plan a short maintenance window.
• 🔗 If you run a validator AND a supernode on the same account, migrate the validator first.

📖 Runbooks:
🛡️ Validators: https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/validator-migration.md
🖥️ Supernodes: https://github.com/LumeraProtocol/lumera/blob/v1.20.0/docs/evm-integration/user-guides/supernode-migration.md
```

---

## ✅ Pre-post checklist

- [ ] 🚨 Keep the **PRELIMINARY** banner on every post while the upgrade block / window-close are `TBD`
- [ ] 🔢 Once decided, replace `TBD at this point` with the real **close block height / date**, confirm the **upgrade block height** is announced and consistent across posts, then publish **final** versions (banner removed)
- [ ] 🔗 Verify the Portal URL is live before pointing users at it (general-public post)
- [ ] 📌 Post order: pinned one-liner (pinned) → 1️⃣ general public → 2️⃣ operators
- [ ] ♻️ Re-use this set for the **mainnet** announcement (swap network names + block/date)
```
