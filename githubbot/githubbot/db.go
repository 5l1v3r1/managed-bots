package githubbot

import (
	"database/sql"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"golang.org/x/oauth2"
)

type DB struct {
	*base.DB
}

func NewDB(db *sql.DB) *DB {
	return &DB{
		DB: base.NewDB(db),
	}
}

// webhook subscription methods

func (d *DB) CreateSubscription(convID chat1.ConvIDStr, repo string, branch string, hookID int64, oauthIdentifier string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO subscriptions
			(conv_id, repo, branch, hook_id, oauth_identifier)
			VALUES
			(?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
			hook_id=VALUES(hook_id)
		`, convID, repo, branch, hookID, oauthIdentifier)
		return err
	})
}

func (d *DB) DeleteSubscription(convID chat1.ConvIDStr, repo string, branch string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscriptions
			WHERE (conv_id = ? AND repo = ? AND branch = ?)
		`, convID, repo, branch)
		return err
	})
}

func (d *DB) DeleteSubscriptionsForRepo(convID chat1.ConvIDStr, repo string) error {
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM subscriptions
			WHERE (conv_id = ? AND repo = ?)
		`, convID, repo)
		return err
	})
}

func (d *DB) GetConvIDsFromRepo(repo string) (res []chat1.ConvIDStr, err error) {
	rows, err := d.DB.Query(`
		SELECT conv_id
		FROM subscriptions
		WHERE repo = ?
		GROUP BY conv_id
	`, repo)
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var convID chat1.ConvIDStr
		if err := rows.Scan(&convID); err != nil {
			return res, err
		}
		res = append(res, convID)
	}
	return res, nil
}

func (d *DB) GetSubscriptionExists(convID chat1.ConvIDStr, repo string, branch string) (exists bool, err error) {
	row := d.DB.QueryRow(`
	SELECT 1
	FROM subscriptions
	WHERE (conv_id = ? AND repo = ? AND branch = ?)
	GROUP BY conv_id
	`, convID, repo, branch)
	var rowRes string
	scanErr := row.Scan(&rowRes)
	switch scanErr {
	case sql.ErrNoRows:
		return false, nil
	case nil:
		return true, nil
	default:
		return false, scanErr
	}
}

func (d *DB) GetSubscriptionForRepoExists(convID chat1.ConvIDStr, repo string) (exists bool, err error) {
	row := d.DB.QueryRow(`
	SELECT 1
	FROM subscriptions
	WHERE (conv_id = ? AND repo = ?)
	`, convID, repo)
	var rowRes string
	err = row.Scan(&rowRes)
	switch err {
	case sql.ErrNoRows:
		return false, nil
	case nil:
		return true, nil
	default:
		return false, err
	}
}

func (d *DB) GetHookIDForRepo(convID chat1.ConvIDStr, repo string) (hookID int64, err error) {
	row := d.DB.QueryRow(`
	SELECT hook_id
	FROM subscriptions
	WHERE (conv_id = ? AND repo = ?)
	`, convID, repo)
	err = row.Scan(&hookID)
	if err != nil {
		return -1, err
	}

	return hookID, nil
}

// OAuth2 token methods

func (d *DB) GetTokenFromConvID(convID chat1.ConvIDStr) (*oauth2.Token, error) {
	var token oauth2.Token
	row := d.DB.QueryRow(`
	SELECT access_token, token_type
	FROM subscriptions
	INNER JOIN oauth ON subscriptions.oauth_identifier = oauth.identifier
	WHERE conv_id = ?
	`, convID)
	err := row.Scan(&token.AccessToken, &token.TokenType)
	switch err {
	case nil:
		return &token, nil
	case sql.ErrNoRows:
		return nil, nil
	default:
		return nil, err
	}
}

func (d *DB) GetToken(identifier string) (*oauth2.Token, error) {
	var token oauth2.Token
	row := d.DB.QueryRow(`SELECT access_token, token_type
		FROM oauth
		WHERE identifier = ?`, identifier)
	err := row.Scan(&token.AccessToken, &token.TokenType)
	switch err {
	case nil:
		return &token, nil
	case sql.ErrNoRows:
		return nil, nil
	default:
		return nil, err
	}
}

func (d *DB) PutToken(identifier string, token *oauth2.Token) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO oauth
		(identifier, access_token, token_type, ctime, mtime)
		VALUES (?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
		access_token=VALUES(access_token),
		mtime=VALUES(mtime)
	`, identifier, token.AccessToken, token.TokenType)
		return err
	})
	return err
}

func (d *DB) DeleteToken(identifier string) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := d.DB.Exec(`DELETE FROM oauth
	WHERE identifier = ?`, identifier)
		return err
	})
	return err
}

// preferences

type UserPreferences struct {
	Mention bool
}

func (d *DB) GetUserPreferences(username string) (*UserPreferences, error) {
	row := d.DB.QueryRow(`SELECT mention
		FROM user_prefs
		WHERE username = ?`, username)
	prefs := &UserPreferences{}
	err := row.Scan(&prefs.Mention)
	switch err {
	case nil:
		return prefs, nil
	case sql.ErrNoRows:
		// if we don't have preferences saved for a user, return default preferences
		return &UserPreferences{
			Mention: true,
		}, nil
	default:
		return nil, err
	}
}

func (d *DB) SetUserPreferences(username string, prefs *UserPreferences) error {
	err := d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO user_prefs 
		(username, mention)
		VALUES (?, ?)
		ON DUPLICATE KEY UPDATE
		mention=VALUES(mention)
	`, username, prefs.Mention)
		return err
	})
	return err
}
