package service

import "time"

type BankAccountInput struct {
	BankCode      string `json:"bank_code" binding:"required,max=20"`
	BranchNumber  string `json:"branch_number" binding:"required,max=20"`
	AccountNumber string `json:"account_number" binding:"required,max=20"`
}

type CreateClinicInput struct {
	TaxIDNumber  string             `json:"tax_id_number" binding:"required,max=32"`
	LegalName    string             `json:"legal_name" binding:"required,max=255"`
	TradeName    *string            `json:"trade_name" binding:"omitempty,max=255"`
	Email        *string            `json:"email" binding:"omitempty,email,max=254"`
	Phone        *string            `json:"phone" binding:"omitempty,max=20"`
	BankAccounts []BankAccountInput `json:"bank_accounts" binding:"required,min=1,dive"`
}

type UpdateClinicInput struct {
	LegalName              *string             `json:"legal_name" binding:"omitempty,max=255"`
	TradeName              *string             `json:"trade_name" binding:"omitempty,max=255"`
	Email                  *string             `json:"email" binding:"omitempty,email,max=254"`
	Phone                  *string             `json:"phone" binding:"omitempty,max=20"`
	BankAccounts           *[]BankAccountInput `json:"bank_accounts" binding:"omitempty,min=1,dive"`
	BankAccountIDsToRemove *[]string           `json:"bank_account_ids_to_remove" binding:"omitempty,min=1,dive"`
}

type CreateDentistInput struct {
	TaxIDNumber           string  `json:"tax_id_number" binding:"required,max=32"`
	LegalName             string  `json:"legal_name" binding:"required,max=255"`
	Email                 *string `json:"email" binding:"omitempty,email,max=254"`
	Phone                 *string `json:"phone" binding:"omitempty,max=20"`
	IsAdmin               bool    `json:"is_admin"`
	IsLegalRepresentative bool    `json:"is_legal_representative"`
}

type UpdateDentistInput struct {
	LegalName *string `json:"legal_name" binding:"omitempty,max=255"`
	Email     *string `json:"email" binding:"omitempty,email,max=254"`
	Phone     *string `json:"phone" binding:"omitempty,max=20"`
}

type UpdateClinicDentistRoleInput struct {
	IsAdmin               *bool `json:"is_admin"`
	IsLegalRepresentative *bool `json:"is_legal_representative"`
}

type LoginInput struct {
	Email    string `json:"email" binding:"required,email,max=254"`
	Password string `json:"password" binding:"required,max=1024"`
}

type BankAccountOutput struct {
	ID            string `json:"id"`
	BankCode      string `json:"bank_code"`
	BranchNumber  string `json:"branch_number"`
	AccountNumber string `json:"account_number"`
}

type DentistOutput struct {
	ID          string  `json:"id"`
	PersonID    string  `json:"person_id"`
	LegalName   string  `json:"legal_name"`
	TaxIDNumber string  `json:"tax_id_number"`
	Email       *string `json:"email,omitempty"`
	Phone       *string `json:"phone,omitempty"`
}

type ClinicDentistOutput struct {
	DentistOutput
	IsAdmin               bool      `json:"is_admin"`
	IsLegalRepresentative bool      `json:"is_legal_representative"`
	StartedAt             time.Time `json:"started_at"`
}

type ClinicOutput struct {
	ID          string   `json:"id"`
	PersonID    string   `json:"person_id"`
	LegalName   string   `json:"legal_name"`
	TradeName   *string  `json:"trade_name,omitempty"`
	TaxIDNumber string   `json:"tax_id_number"`
	Email       *string  `json:"email,omitempty"`
	Phone       *string  `json:"phone,omitempty"`
	DentistIDs  []string `json:"dentist_ids"`
}

type ClinicDetailsOutput struct {
	ClinicOutput
	BankAccounts []BankAccountOutput `json:"bank_accounts"`
}

type LoginOutput struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
}
