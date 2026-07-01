package handler

type APIVisibility string

const (
	APIPublic  APIVisibility = "public"
	APIPrivate APIVisibility = "private"
)

type APIRoute struct {
	Visibility  APIVisibility
	Transport   string
	Method      string
	Path        string
	Name        string
	Handler     string
	Description string
}

func PublicAPIRoutes() []APIRoute {
	return []APIRoute{
		{APIPublic, "http", "GET", "/health", "health", "HealthHandler.Routes", "Liveness probe."},
		{APIPublic, "http", "GET", "/ready", "ready", "HealthHandler.ready", "Readiness probe for dependencies."},
		{APIPublic, "http", "GET", "/phones", "phones.list", "PhonesHTTP.listOrAdd", "List phones and FSM stats."},
		{APIPublic, "http", "POST", "/phones", "phones.add", "PhonesHTTP.listOrAdd", "Add a phone to the farm."},
		{APIPublic, "http", "GET", "/phones/{serial}", "phones.get", "PhonesHTTP.phoneBySerial", "Read one phone card."},
		{APIPublic, "http", "POST", "/phones/{serial}/add", "phones.add_by_path", "PhonesHTTP.phoneBySerial", "Add a phone by path serial."},
		{APIPublic, "http", "POST", "/phones/{serial}/remove", "phones.retire", "PhonesHTTP.phoneBySerial", "Retire a phone."},
		{APIPublic, "http", "POST", "/phones/{serial}/pause", "phones.pause", "PhonesHTTP.phoneBySerial", "Pause a phone with reason."},
		{APIPublic, "http", "POST", "/phones/{serial}/resume", "phones.resume", "PhonesHTTP.phoneBySerial", "Resume a paused phone."},
		{APIPublic, "http", "POST", "/phones/{serial}/reprovision", "phones.reprovision", "PhonesHTTP.phoneBySerial", "Reset phone to new for setup."},
		{APIPublic, "http", "PATCH/PUT", "/phones/{serial}/stand-seq", "phones.set_stand_seq", "PhonesHTTP.phoneBySerial", "Update stand sequence number."},
		{APIPublic, "http", "POST", "/phones/{serial}/stand-seq/sync-from-home", "phones.sync_stand_seq_from_home", "PhonesHTTP.phoneBySerial", "Read stand sequence number from the phone home screen and persist it."},
		{APIPublic, "http", "GET", "/stats", "phones.stats", "PhonesHTTP.stats", "Read FSM counters."},
		{APIPublic, "http", "GET", "/phones/{serial}/screen", "observer.screen", "PhonesHTTP.captureScreen", "Proxy screenshot capture."},
		{APIPublic, "http", "GET", "/phones/{serial}/ui", "observer.ui", "PhonesHTTP.dumpUI", "Proxy UI dump."},
		{APIPublic, "http", "GET", "/phones/{serial}/observe", "observer.observe", "PhonesHTTP.observe", "Proxy screenshot and UI dump together."},
		{APIPublic, "http", "POST", "/phones/{serial}/tap", "executor.tap", "PhonesHTTP.executorTap", "Proxy tap action."},
		{APIPublic, "http", "POST", "/phones/{serial}/swipe", "executor.swipe", "PhonesHTTP.executorSwipe", "Proxy swipe action."},
		{APIPublic, "http", "POST", "/phones/{serial}/type", "executor.type_text", "PhonesHTTP.executorType", "Proxy text input action."},
		{APIPublic, "http", "POST", "/phones/{serial}/key", "executor.key", "PhonesHTTP.executorKey", "Proxy Android key action."},
		{APIPublic, "http", "POST", "/phones/{serial}/wifi", "connector.wifi", "PhonesHTTP.phoneWifi", "Enable or disable WiFi through connector."},
		{APIPublic, "http", "GET", "/phones/{serial}/content", "content.list", "PhonesHTTP.phoneContent", "List phone content."},
		{APIPublic, "http", "POST", "/phones/{serial}/content/register", "content.register", "PhonesHTTP.phoneContent", "Register MinIO object for phone."},
		{APIPublic, "http", "POST", "/phones/{serial}/content/download", "content.download", "PhonesHTTP.phoneContent", "Start content delivery to phone."},
		{APIPublic, "http", "DELETE", "/phones/{serial}/content", "content.delete_all", "PhonesHTTP.phoneContent", "Delete all content for phone."},
		{APIPublic, "http", "DELETE", "/phones/{serial}/content/device", "content.delete_device", "PhonesHTTP.phoneContent", "Delete device-side content only."},
		{APIPublic, "http", "DELETE", "/phones/{serial}/content/storage", "content.delete_storage", "PhonesHTTP.phoneContent", "Delete storage-side content only."},
		{APIPublic, "http", "DELETE", "/phones/{serial}/content/{content_id}", "content.delete_one", "PhonesHTTP.phoneContent", "Delete one content item."},
		{APIPublic, "http", "GET", "/phones/{serial}/contacts", "contacts.list", "PhonesHTTP.phoneContacts", "List contacts."},
		{APIPublic, "http", "POST", "/phones/{serial}/contacts/upload", "contacts.upload", "PhonesHTTP.phoneContacts", "Import contacts."},
		{APIPublic, "http", "POST", "/phones/{serial}/contacts/sync", "contacts.sync", "PhonesHTTP.phoneContacts", "Run contacts sync."},
		{APIPublic, "http", "POST", "/phones/{serial}/contacts/merge", "contacts.merge", "PhonesHTTP.phoneContacts", "Merge duplicate contacts."},
		{APIPublic, "http", "POST", "/phones/{serial}/contacts/groups", "contacts.groups", "PhonesHTTP.phoneContacts", "Apply contact groups."},
		{APIPublic, "http", "GET", "/phones/{serial}/contacts/export", "contacts.export", "PhonesHTTP.phoneContacts", "Export contacts as vCard."},
		{APIPublic, "http", "DELETE", "/phones/{serial}/contacts/{contact_id}", "contacts.delete", "PhonesHTTP.phoneContacts", "Delete one contact."},
		{APIPublic, "http", "POST", "/phones/{serial}/video/screenshots", "video.screenshots", "PhonesHTTP.phoneVideo", "Create video from screenshots."},
		{APIPublic, "http", "POST", "/phones/{serial}/video/ai", "video.ai", "PhonesHTTP.phoneVideo", "Generate AI video."},
		{APIPublic, "http", "POST", "/phones/{serial}/video/edit", "video.edit", "PhonesHTTP.phoneVideo", "Edit a video."},
		{APIPublic, "http", "GET", "/phones/{serial}/video/jobs/{id}", "video.job", "PhonesHTTP.phoneVideo", "Read video job state."},
		{APIPublic, "http", "DELETE", "/phones/{serial}/video/jobs/{id}", "video.delete", "PhonesHTTP.phoneVideo", "Delete video output."},
		{APIPublic, "http", "POST", "/recovery/run", "recovery.run", "OrchestratorHandler.RunRecoveryHTTP", "Debug recovery flow."},
		{APIPublic, "http", "POST", "/recovery/outcome", "recovery.outcome", "OrchestratorHandler.ReportOutcomeHTTP", "Report recovery outcome."},
		{APIPublic, "grpc", "rpc", "/orchestrator.v1.OrchestratorService/RunRecovery", "recovery.grpc.run", "OrchestratorHandler.RunRecovery", "Run recovery flow through gRPC."},
		{APIPublic, "grpc", "rpc", "/orchestrator.v1.OrchestratorService/ReportRecoveryOutcome", "recovery.grpc.outcome", "OrchestratorHandler.ReportRecoveryOutcome", "Report recovery outcome through gRPC."},
	}
}

func PrivateAPIRoutes() []APIRoute {
	return []APIRoute{
		{APIPrivate, "http-out", "GET", "observer:/screen/{serial}", "observer.screen", "observer client", "Capture screenshots."},
		{APIPrivate, "http-out", "GET", "observer:/ui/{serial}", "observer.ui", "observer client", "Dump UI XML."},
		{APIPrivate, "http-out", "POST", "observer:/detect-state", "observer.detect_state", "observer client", "Read screenshot OCR/VLM text for stand number fallback."},
		{APIPrivate, "grpc-out", "CALL", "executor.ExecutorService", "executor.actions", "executor client", "Run tap, swipe, key, and recovery plan actions."},
		{APIPrivate, "grpc-out", "CALL", "connector.ConnectorService", "connector", "connector client", "Control WiFi and provision connectivity."},
		{APIPrivate, "http-out", "POST/GET", "phone-provisioner", "provisioner", "provision client", "Start and poll provision runs."},
		{APIPrivate, "http/nats-out", "CALL/PUB", "content-distributor, af.content.*", "content", "content client", "Register, download, list, and delete content."},
		{APIPrivate, "grpc-out", "CALL", "contacts.ContactsService", "contacts", "contacts client", "Manage contacts."},
		{APIPrivate, "grpc-out", "CALL", "video.VideoService", "video", "video client", "Manage video jobs."},
		{APIPrivate, "nats", "REQ/RESP/PUB", "af.recovery.request, af.recovery.response, af.recovery.outcome", "recovery.nats", "recovery client", "Ask recovery-engine for plans and publish outcomes."},
		{APIPrivate, "nats-out", "PUB", "phone.state.changed", "fsm.events", "phone service", "Publish phone FSM transitions."},
		{APIPrivate, "sql/memory", "READ/WRITE", "phones", "store.phones", "repository", "Persist phone state."},
	}
}

func AllAPIRoutes() []APIRoute {
	routes := append([]APIRoute{}, PublicAPIRoutes()...)
	return append(routes, PrivateAPIRoutes()...)
}
