// Package status contains status codes not unline the http
// statuses, but tailored toward the ddn ecosystem
package status

// Labels contains the labels of the statuses.
var Labels map[int]string

func init() {
	Labels = make(map[int]string)

	// Info
	Labels[Started] = "Started"
	Labels[InProgress] = "In Progress"
	Labels[DownloadInProgress] = "Downloading"
	Labels[UploadInProgress] = "Uploading"
	Labels[ExtractingArchive] = "Extracting Archive"
	Labels[ArchivingDump] = "Zipping dump"
	Labels[ValidatingDump] = "Validating Dump"
	Labels[ImportInProgress] = "Importing"
	Labels[ExportInProgress] = "Exporting"
	Labels[CopyInProgress] = "Copying"

	// Success
	Labels[Success] = "Completed"
	Labels[Created] = "Created"
	Labels[Accepted] = "Accepted"
	Labels[Update] = "Update"

	// Client Error
	Labels[ClientError] = "Client Error"
	Labels[NotFound] = "Not found"
	Labels[DownloadFailed] = "Download failed"
	Labels[ArchiveNotSupported] = "Archive not suppported"
	Labels[MultipleFilesInArchive] = "Archive contains multiple files"
	Labels[MissingParameters] = "Missing Parameters"
	Labels[InvalidJSON] = "Invalid JSON Request"

	// Server Error
	Labels[ServerError] = "Server Error"
	Labels[ExtractingArchiveFailed] = "Extracting archive failed"
	Labels[ValidationFailed] = "Validation failed"
	Labels[ImportFailed] = "Import failed"
	Labels[ExportFailed] = "Export failed"
	Labels[CreateDatabaseFailed] = "Creating database failed"
	Labels[ListDatabaseFailed] = "Listing databases failed"
	Labels[DropDatabaseFailed] = "Dropping database failed"
	Labels[ZippingDumpFailed] = "Zipping dump failed"

	// Warnings
	Labels[DropInProgress] = "Drop in progress"
	Labels[RemovalScheduled] = "Removal scheduled"
}

// Info statuses are used to convey that something has happened
// but has not finished yet. It is not a success, nor a failure.
//
// They can range from 1 to 99
const (
	Started    int = 1 // status.Started
	InProgress int = 2 // status.InProgress

	DownloadInProgress int = 3  // status.DownloadInProgress
	ExtractingArchive  int = 4  // status.ExtractingArchive
	ValidatingDump     int = 5  // status.ValidatingDump
	ImportInProgress   int = 6  // status.ImportInProgress
	CopyInProgress     int = 7  // status.CopyInProgress
	ExportInProgress   int = 8  // status.ExportInProgress
	UploadInProgress   int = 9  // status.UploadInProgress
	ArchivingDump      int = 10 // status.ArchivingDump
)

// Success statuses are used to convey a successful result.
const (
	Success  int = 100 // status.Success
	Created  int = 101 // status.Created
	Accepted int = 102 // status.Accepted
	Update   int = 103 // status.Update
)

// Client errors are used to convey that something was
// wrong with a client request.
const (
	ClientError            int = 200 // status.ClientError
	NotFound               int = 201 // status.NotFound
	DownloadFailed         int = 202 // status.DownloadFailed
	ArchiveNotSupported    int = 203 // status.ArchiveNotSupported
	MultipleFilesInArchive int = 204 // status.MultipleFilesInArchive
	MissingParameters      int = 205 // status.MissingParameters
	InvalidJSON            int = 206 // status.InvalidJSON
)

// Server errors are used to convey that something went wrong
// on the server.
const (
	ServerError              int = 300 // status.ServerError
	ExtractingArchiveFailed  int = 302 // status.ExtractingArchiveFailed
	ValidationFailed         int = 303 // status.ValidationFailed
	ImportFailed             int = 304 // status.ImportFailed
	CreateDatabaseFailed     int = 305 // status.CreateDatabaseFailed
	ListDatabaseFailed       int = 306 // status.ListDatabaseFailed
	DropDatabaseFailed       int = 307 // status.DropDatabaseFailed
	SaveSubscriptionFailed   int = 308 // status.SaveSubscriptionFailed
	DeleteSubscriptionFailed int = 309 // status.DeleteSubscriptionFailed
	ExportFailed             int = 310 // status.ExportFailed
	ZippingDumpFailed        int = 311 // status.ZippingDumpFailed
)

// Warnings are for issuing warnings.
const (
	RemovalScheduled int = 400 // status.RemovalScheduled
	DropInProgress   int = 401 // status.DropInProgress
)
