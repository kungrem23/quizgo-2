package quiz

import "context"

// "database/sql"
// "fmt"
// "log"

// type UserRepo struct {
// 	db *sql.DB
// }

// func NewUserRepo(db *sql.DB) *UserRepo {
// 	return &UserRepo{db: db}
// }

func (r *PostgresRepository) CreateNewUser(ctx context.Context, username string, passwordHash string) error {
	query := `INSERT INTO users
	(username, password_hash)
	VALUES ($1, $2)
	RETURNING id, username, password_hash;`
	_, err := r.db.ExecContext(ctx, query, username, passwordHash)
	// user := NewUser()
	// err := row.Scan(&user.Id, &user.Username, &user.PasswordHash)
	// if err != nil {
	// 	log.Printf("Adding user error: %v\n", err)
	// 	return nil, err
	// }
	return err
}

func (r *PostgresRepository) GetAllUsers(ctx context.Context) ([]User, error) {
	query := `SELECT id, username, password_hash FROM users`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		// log.Printf("Selecting all users error: %v\n", err)
		return nil, err
	}
	var users []User
	defer rows.Close()
	for rows.Next() {
		var u User
		err := rows.Scan(&u.Id, &u.Username, &u.PasswordHash)
		if err != nil {
			// log.Printf("Scanning user error: %v\n", err)
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (r *PostgresRepository) GetUser(ctx context.Context, id int) (User, error) {
	query := `SELECT id, username, password_hash FROM users
	WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	var user User
	err := row.Scan(&user.Id, &user.Username, &user.PasswordHash)
	// if err != nil {
	// 	log.Printf("Selecting user(id=%v) error: %v\n", id, err)
	// 	return nil, err
	// }
	return user, err
}

func (r *PostgresRepository) GetUserByUsername(ctx context.Context, username string) (User, error) {
	query := `SELECT id, username, password_hash FROM users
	WHERE username=$1`
	row := r.db.QueryRowContext(ctx, query, username)
	var user User
	err := row.Scan(&user.Id, &user.Username, &user.PasswordHash)
	// if err != nil {
	// 	log.Printf("Selecting user by username(%v) error: %v", username, err)
	// 	return nil, err
	// }
	return user, err
}

func (r *PostgresRepository) DeleteUser(ctx context.Context, id int) error {
	query := "DELETE FROM users WHERE id = $1;"
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	// if err != nil {
	// 	// log.Printf("Deleting user error: %v\n", err)
	// 	return err
	// }
	return err
}
