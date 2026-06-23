# Equipment Config

## Directory layout

The server loads all `.json` files recursively under `config/equipment`.

Example layout:

- `config/equipment/weapon/`
- `config/equipment/armor/`
- `config/equipment/ring/`
- `config/equipment/shoes/`
- `config/equipment/accessory/`

You can keep splitting files as the list grows, for example:

- `config/equipment/weapon/sword.json`
- `config/equipment/weapon/staff.json`
- `config/equipment/ring/basic_rings.json`

## Rules

- Every equipment `id` must be globally unique across all files.
- Every JSON file should be an object whose keys are equipment IDs.
- The file name does not affect behavior. Only the JSON content matters.
- The directory name does not affect behavior. It is only for organization.
- If `id` is omitted inside an entry, the outer object key is used.
- If `slotCount` is omitted or `<= 0`, it defaults to `1`.
- If `occupiesSlots` is omitted, the server auto-generates it from `slot` and `slotCount`.

## Supported slots

Common slots currently used by the project:

- `weapon_main`
- `weapon_sub`
- `armor`
- `shoes`
- `ring`
- `accessory`

## Auto-generated occupiesSlots

When `occupiesSlots` is not configured, the server fills it like this:

- `ring` + `slotCount: 1` -> `["ring1"]`
- `ring` + `slotCount: 2` -> `["ring1", "ring2"]`
- `weapon_main` + `slotCount: 1` -> `["weapon_main"]`
- `weapon_main` + `slotCount: 2` -> `["weapon_main", "weapon_sub"]`
- `weapon_sub` -> `["weapon_sub"]`
- other slots -> `[slot]`

## JSON shape

Each file should look like this:

```json
{
  "bronze_sword": {
    "id": "bronze_sword",
    "name": "Bronze Sword",
    "slot": "weapon_main",
    "stat": {
      "strength": 5,
      "hpMax": 40
    },
    "combatStat": {
      "physicalAttackMin": 7,
      "physicalAttackMax": 12,
      "attackStartupMs": -20,
      "attackRecoveryMs": -30,
      "attackIntervalMs": -50
    },
    "requirement": {
      "strength": 10
    },
    "minLevel": 5,
    "allowedJobs": ["warrior"]
  }
}
```

## Equipment fields

### Top-level fields

- `id`: equipment ID. Should match the outer object key.
- `name`: display name.
- `slot`: equipment slot.
- `slotCount`: how many slot positions this equipment occupies.
- `occupiesSlots`: explicit occupied slots. Optional.
- `stat`: base attribute bonus.
- `combatStat`: combat attribute bonus.
- `requirement`: required base attributes to equip.
- `minLevel`: minimum required level.
- `allowedJobs`: allowed job codes.
- `allowedGenders`: allowed genders.
- `exclusiveGroup`: only one item in the same exclusive group can be equipped.
- `incompatibleWith`: explicit incompatible equipment IDs.

### stat fields

- `strength`
- `intelligence`
- `agility`
- `luck`
- `hp`
- `mp`
- `hpMax`
- `mpMax`

### requirement fields

- `strength`
- `intelligence`
- `agility`
- `luck`

### combatStat fields

- `physicalAttackMin`
- `physicalAttackMax`
- `magicAttackMin`
- `magicAttackMax`
- `physicalDefense`
- `magicDefense`
- `moveSpeed`
- `evasion`
- `accuracy`
- `critRate`
- `critDamage`
- `hpRecovery`
- `mpRecovery`
- `castSpeed`
- `attackStartupMs`
- `attackActiveMs`
- `attackRecoveryMs`
- `attackIntervalMs`

## Notes

- `moveSpeed` is still clamped by server combat calculation.
- Negative combat rhythm values are allowed for equipment, such as reducing `attackRecoveryMs`.
- If two files define the same equipment ID, server startup fails.
- If a file contains invalid JSON, server startup fails.

## Recommended workflow for adding equipment

1. Choose the slot directory.
2. Put the equipment into an existing JSON file or create a new one.
3. Keep the outer key and inner `id` the same.
4. Make sure the `id` does not exist anywhere else.
5. Restart the server and verify equip behavior in game.
