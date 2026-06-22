package scene

import "math"

const recoveryTickMS int64 = 1000

func applyRecovery(player Player, dt float64) Player {
	if dt <= 0 {
		return player
	}

	elapsedMS := int64(math.Round(dt * 1000))
	if elapsedMS <= 0 {
		return player
	}

	player.RecoveryElapsedMS += elapsedMS
	if player.RecoveryElapsedMS < recoveryTickMS {
		return player
	}

	ticks := player.RecoveryElapsedMS / recoveryTickMS
	if ticks <= 0 {
		return player
	}
	player.RecoveryElapsedMS -= ticks * recoveryTickMS

	final := player.Stat.Final
	if player.CombatStat.HPRecovery > 0 && final.HP < final.HPMax {
		player.HPRecoveryCarry += float64(ticks) * player.CombatStat.HPRecovery
		amount := int32(math.Floor(player.HPRecoveryCarry))
		if amount > 0 {
			player.HPRecoveryCarry -= float64(amount)
			player.Stat.Base.HP = minInt32(player.Stat.Base.HP+amount, final.HPMax)
		}
	} else {
		player.HPRecoveryCarry = 0
	}

	if player.CombatStat.MPRecovery > 0 && final.MP < final.MPMax {
		player.MPRecoveryCarry += float64(ticks) * player.CombatStat.MPRecovery
		amount := int32(math.Floor(player.MPRecoveryCarry))
		if amount > 0 {
			player.MPRecoveryCarry -= float64(amount)
			player.Stat.Base.MP = minInt32(player.Stat.Base.MP+amount, final.MPMax)
		}
	} else {
		player.MPRecoveryCarry = 0
	}

	return player
}
