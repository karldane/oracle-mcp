package presidio

type EntityType string

const (
	EntityEmailAddress   EntityType = "EMAIL_ADDRESS"
	EntityPhoneNumber    EntityType = "PHONE_NUMBER"
	EntityUkPostcode     EntityType = "UK_POSTCODE"
	EntityUkNino         EntityType = "UK_NINO"
	EntityUkNhsNumber    EntityType = "UK_NHS_NUMBER"
	EntityCreditCard     EntityType = "CREDIT_CARD"
	EntityIban           EntityType = "IBAN"
	EntityIPv4           EntityType = "IP_ADDRESS_V4"
	EntityIPv6           EntityType = "IP_ADDRESS_V6"
	EntityDateOfBirth    EntityType = "DATE_OF_BIRTH"
	EntityPerson         EntityType = "PERSON"
	EntityUsSsn          EntityType = "US_SSN"
	EntityPassportNumber EntityType = "PASSPORT_NUMBER"
	EntityDriverLicence  EntityType = "DRIVER_LICENCE"
)

type RecognizerResult struct {
	EntityType EntityType
	Start      int
	End        int
	Score      float64
	Recognizer string
}

type PIIReport struct {
	Detected        bool
	Entities        []RecognizerResult
	Treatment       TreatmentKind
	TreatmentReason string
	OriginalLength  int
	TruncatedAt     int
}

type TreatmentKind string

const (
	TreatmentNone              TreatmentKind = "none"
	TreatmentRedacted          TreatmentKind = "redacted"
	TreatmentHashed            TreatmentKind = "hashed"
	TreatmentPseudonymised     TreatmentKind = "pseudonymised"
	TreatmentMasked            TreatmentKind = "masked"
	TreatmentTruncated         TreatmentKind = "truncated"
	TreatmentTruncatedRedacted TreatmentKind = "truncated_and_redacted"
	TreatmentStripped          TreatmentKind = "stripped"
)
