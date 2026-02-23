package service

import "time"

type BankAccountInput struct {
	BankCode      string `json:"bank_code" binding:"required"`
	BranchNumber  string `json:"branch_number" binding:"required"`
	AccountNumber string `json:"account_number" binding:"required"`
}

type CreateClinicInput struct {
	TaxIDNumber  string             `json:"tax_id_number" binding:"required"`
	LegalName    string             `json:"legal_name" binding:"required"`
	TradeName    *string            `json:"trade_name"`
	Email        *string            `json:"email" binding:"omitempty,email"`
	Phone        *string            `json:"phone"`
	BankAccounts []BankAccountInput `json:"bank_accounts" binding:"required,min=1,dive"`
}

type UpdateClinicInput struct {
	LegalName              *string             `json:"legal_name"`
	TradeName              *string             `json:"trade_name"`
	Email                  *string             `json:"email" binding:"omitempty,email"`
	Phone                  *string             `json:"phone"`
	BankAccounts           *[]BankAccountInput `json:"bank_accounts" binding:"omitempty,min=1,dive"`
	BankAccountIDsToRemove *[]string           `json:"bank_account_ids_to_remove" binding:"omitempty,min=1,dive"`
}

type CreateDentistInput struct {
	TaxIDNumber           string  `json:"tax_id_number" binding:"required"`
	LegalName             string  `json:"legal_name" binding:"required"`
	Email                 *string `json:"email" binding:"omitempty,email"`
	Phone                 *string `json:"phone"`
	IsAdmin               bool    `json:"is_admin"`
	IsLegalRepresentative bool    `json:"is_legal_representative"`
}

type UpdateDentistInput struct {
	LegalName *string `json:"legal_name"`
	Email     *string `json:"email" binding:"omitempty,email"`
	Phone     *string `json:"phone"`
}

type UpdateClinicDentistRoleInput struct {
	IsAdmin               *bool `json:"is_admin"`
	IsLegalRepresentative *bool `json:"is_legal_representative"`
}

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
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
