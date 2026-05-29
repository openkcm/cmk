package cmkapi

import "github.com/getkin/kin-openapi/openapi3"

func SwaggerUI(swagger *openapi3.T) (string, error) {
	jsonBytes, err := swagger.MarshalJSON()
	if err != nil {
		return "", err
	}

	spec := `
		<!DOCTYPE html>
		<html lang="en">

		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>CMK Swagger UI</title>
			<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.32.5/swagger-ui.css">
		</head>

		<body>
			<div id="swagger-ui"></div>
			<script src="https://unpkg.com/swagger-ui-dist@5.32.5/swagger-ui-bundle.js"></script>
			<script>
				const spec = ` + string(jsonBytes) + `;
				window.onload = () => {
					window.ui = SwaggerUIBundle({
						spec: spec,
						dom_id: '#swagger-ui', 
					})
				};
			</script>
		</body>

		</html>
	`
	return spec, nil
}
