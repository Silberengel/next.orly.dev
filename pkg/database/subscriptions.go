package database

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"encoders.orly/json"
	"github.com/dgraph-io/badger/v4"
)

type Subscription struct {
	TrialEnd  time.Time `json:"trial_end"`
	PaidUntil time.Time `json:"paid_until"`
}

func (d *D) GetSubscription(pubkey []byte) (*Subscription, error) {
	key := fmt.Sprintf("sub:%s", hex.EncodeToString(pubkey))
	var sub *Subscription

	err := d.DB.View(
		func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(key))
			if errors.Is(err, badger.ErrKeyNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			return item.Value(
				func(val []byte) error {
					sub = &Subscription{}
					return json.Unmarshal(val, sub)
				},
			)
		},
	)
	return sub, err
}

func (d *D) IsSubscriptionActive(pubkey []byte) (bool, error) {
	key := fmt.Sprintf("sub:%s", hex.EncodeToString(pubkey))
	now := time.Now()
	active := false

	err := d.DB.Update(
		func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(key))
			if err == badger.ErrKeyNotFound {
				sub := &Subscription{TrialEnd: now.AddDate(0, 0, 30)}
				data, err := json.Marshal(sub)
				if err != nil {
					return err
				}
				active = true
				return txn.Set([]byte(key), data)
			}
			if err != nil {
				return err
			}

			var sub Subscription
			err = item.Value(
				func(val []byte) error {
					return json.Unmarshal(val, &sub)
				},
			)
			if err != nil {
				return err
			}

			active = now.Before(sub.TrialEnd) || (!sub.PaidUntil.IsZero() && now.Before(sub.PaidUntil))
			return nil
		},
	)
	return active, err
}

func (d *D) ExtendSubscription(pubkey []byte, days int) error {
	if days <= 0 {
		return fmt.Errorf("invalid days: %d", days)
	}

	key := fmt.Sprintf("sub:%s", hex.EncodeToString(pubkey))
	now := time.Now()

	return d.DB.Update(
		func(txn *badger.Txn) error {
			var sub Subscription
			item, err := txn.Get([]byte(key))
			if err == badger.ErrKeyNotFound {
				sub.PaidUntil = now.AddDate(0, 0, days)
			} else if err != nil {
				return err
			} else {
				err = item.Value(
					func(val []byte) error {
						return json.Unmarshal(val, &sub)
					},
				)
				if err != nil {
					return err
				}
				extendFrom := now
				if !sub.PaidUntil.IsZero() && sub.PaidUntil.After(now) {
					extendFrom = sub.PaidUntil
				}
				sub.PaidUntil = extendFrom.AddDate(0, 0, days)
			}

			data, err := json.Marshal(&sub)
			if err != nil {
				return err
			}
			return txn.Set([]byte(key), data)
		},
	)
}

type Payment struct {
	Amount    int64     `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
	Invoice   string    `json:"invoice"`
	Preimage  string    `json:"preimage"`
}

func (d *D) RecordPayment(
	pubkey []byte, amount int64, invoice, preimage string,
) error {
	now := time.Now()
	key := fmt.Sprintf("payment:%d:%s", now.Unix(), hex.EncodeToString(pubkey))

	payment := Payment{
		Amount:    amount,
		Timestamp: now,
		Invoice:   invoice,
		Preimage:  preimage,
	}

	data, err := json.Marshal(&payment)
	if err != nil {
		return err
	}

	return d.DB.Update(
		func(txn *badger.Txn) error {
			return txn.Set([]byte(key), data)
		},
	)
}

func (d *D) GetPaymentHistory(pubkey []byte) ([]Payment, error) {
	prefix := fmt.Sprintf("payment:")
	suffix := fmt.Sprintf(":%s", hex.EncodeToString(pubkey))
	var payments []Payment

	err := d.DB.View(
		func(txn *badger.Txn) error {
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()

			for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
				key := string(it.Item().Key())
				if !strings.HasSuffix(key, suffix) {
					continue
				}

				err := it.Item().Value(
					func(val []byte) error {
						var payment Payment
						err := json.Unmarshal(val, &payment)
						if err != nil {
							return err
						}
						payments = append(payments, payment)
						return nil
					},
				)
				if err != nil {
					return err
				}
			}
			return nil
		},
	)

	return payments, err
}
