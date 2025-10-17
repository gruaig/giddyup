// This is the working OLD method from fetch_all that needs to be copied into autoupdate.go
// REPLACE the current insertToDatabase method starting at line 359

func (s *AutoUpdateService) insertToDatabase(dateStr string, races []scraper.Race, prelim bool) (int, int, error) {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	// Set performance knobs for this transaction
	tx.Exec(`SET LOCAL synchronous_commit = off`) // Safe for batch ETL
	tx.Exec(`SET LOCAL statement_timeout = 0`)

	// Use OLD reliable method - individual INSERT queries
	log.Println("[AutoUpdate]      • Upserting dimensions (OLD method)...")
	
	// Collect unique entities
	courses := make(map[string]string)
	horses := make(map[string]bool)
	trainers := make(map[string]bool)
	jockeys := make(map[string]bool)
	owners := make(map[string]bool)

	for _, race := range races {
		if race.Course != "" {
			courses[race.Course] = race.Region
		}
		for _, runner := range race.Runners {
			if runner.Horse != "" {
				horses[runner.Horse] = true
			}
			if runner.Trainer != "" {
				trainers[runner.Trainer] = true
			}
			if runner.Jockey != "" {
				jockeys[runner.Jockey] = true
			}
			if runner.Owner != "" {
				owners[runner.Owner] = true
			}
		}
	}

	// Insert dimensions
	for courseName, region := range courses {
		_, err := tx.Exec(`INSERT INTO racing.courses (course_name, region) VALUES ($1, $2) ON CONFLICT ON CONSTRAINT courses_uniq DO NOTHING`, courseName, region)
		if err != nil {
			return 0, 0, err
		}
	}

	for horse := range horses {
		_, err := tx.Exec(`INSERT INTO racing.horses (horse_name) VALUES ($1) ON CONFLICT ON CONSTRAINT horses_uniq DO NOTHING`, horse)
		if err != nil {
			return 0, 0, err
		}
	}

	for trainer := range trainers {
		_, err := tx.Exec(`INSERT INTO racing.trainers (trainer_name) VALUES ($1) ON CONFLICT ON CONSTRAINT trainers_uniq DO NOTHING`, trainer)
		if err != nil {
			return 0, 0, err
		}
	}

	for jockey := range jockeys {
		_, err := tx.Exec(`INSERT INTO racing.jockeys (jockey_name) VALUES ($1) ON CONFLICT ON CONSTRAINT jockeys_uniq DO NOTHING`, jockey)
		if err != nil {
			return 0, 0, err
		}
	}

	for owner := range owners {
		_, err := tx.Exec(`INSERT INTO racing.owners (owner_name) VALUES ($1) ON CONFLICT ON CONSTRAINT owners_uniq DO NOTHING`, owner)
		if err != nil {
			return 0, 0, err
		}
	}

	// Populate foreign keys
	log.Println("[AutoUpdate]      • Populating foreign keys (OLD method)...")
	
	courseIDs := make(map[string]int64)
	horseIDs := make(map[string]int64)
	trainerIDs := make(map[string]int64)
	jockeyIDs := make(map[string]int64)
	ownerIDs := make(map[string]int64)

	// Look up all IDs
	for _, race := range races {
		if race.Course != "" && courseIDs[race.Course] == 0 {
			var id int64
			tx.QueryRow(`SELECT course_id FROM racing.courses WHERE racing.norm_text(course_name) = racing.norm_text($1)`, race.Course).Scan(&id)
			courseIDs[race.Course] = id
		}

		for _, runner := range race.Runners {
			if runner.Horse != "" && horseIDs[runner.Horse] == 0 {
				var id int64
				tx.QueryRow(`SELECT horse_id FROM racing.horses WHERE racing.norm_text(horse_name) = racing.norm_text($1)`, runner.Horse).Scan(&id)
				horseIDs[runner.Horse] = id
			}
			if runner.Trainer != "" && trainerIDs[runner.Trainer] == 0 {
				var id int64
				tx.QueryRow(`SELECT trainer_id FROM racing.trainers WHERE racing.norm_text(trainer_name) = racing.norm_text($1)`, runner.Trainer).Scan(&id)
				trainerIDs[runner.Trainer] = id
			}
			if runner.Jockey != "" && jockeyIDs[runner.Jockey] == 0 {
				var id int64
				tx.QueryRow(`SELECT jockey_id FROM racing.jockeys WHERE racing.norm_text(jockey_name) = racing.norm_text($1)`, runner.Jockey).Scan(&id)
				jockeyIDs[runner.Jockey] = id
			}
			if runner.Owner != "" && ownerIDs[runner.Owner] == 0 {
				var id int64
				tx.QueryRow(`SELECT owner_id FROM racing.owners WHERE racing.norm_text(owner_name) = racing.norm_text($1)`, runner.Owner).Scan(&id)
				ownerIDs[runner.Owner] = id
			}
		}
	}

	// Populate foreign keys in the race data
	for i := range races {
		races[i].CourseID = int(courseIDs[races[i].Course])

		for j := range races[i].Runners {
			races[i].Runners[j].HorseID = int(horseIDs[races[i].Runners[j].Horse])
			races[i].Runners[j].TrainerID = int(trainerIDs[races[i].Runners[j].Trainer])
			races[i].Runners[j].JockeyID = int(jockeyIDs[races[i].Runners[j].Jockey])
			races[i].Runners[j].OwnerID = int(ownerIDs[races[i].Runners[j].Owner])
		}
	}

	// Verify
	horsesPopulated := 0
	totalRunners := 0
	for _, race := range races {
		totalRunners += len(race.Runners)
		for _, runner := range race.Runners {
			if runner.HorseID > 0 {
				horsesPopulated++
			}
		}
	}
	log.Printf("[AutoUpdate]      ✓ Verified: %d/%d runners have horse_id", horsesPopulated, totalRunners)

	// Now insert races and runners with the populated IDs...
	// (rest of the method continues with race/runner INSERT logic)

