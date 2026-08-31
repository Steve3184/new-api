package system_setting

var ServerAddress = "http://localhost:3000"
var TaskPublicAddress = ""
var WorkerUrl = ""
var WorkerValidKey = ""
var WorkerAllowHttpImageRequestEnabled = false
var WorkerMeshyImageProxyEnabled = false
var WorkerMeshyImageProxyBaseURL = ""
var WorkerMeshyImageProxyAPIKey = ""

func EnableWorker() bool {
	return WorkerUrl != ""
}
