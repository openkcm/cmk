package cert

func ExportGenerateTemporaryCertAndKeyWithSettings() func(
	certCreator CertificateCreator, pemEncoder PEMEncoder) (string, string, error) {
	return generateTempCertKeyPairWithCustomProviders
}
