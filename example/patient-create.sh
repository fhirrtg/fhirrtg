curl -X POST "http://localhost:8888/Patient" -H "Content-Type: application/fhir+json" -d '{
  "address": [
    {
      "city": "Vancouver",
      "country": "CAN",
      "line": [
        "N EW NEW NEW"
      ],
      "state": "British Columbia",
      "type": "physical",
      "use": "home"
    }
  ],
  "communication": [
    {
      "language": {
        "coding": [
          {
            "code": "en",
            "system": "http://hl7.org/fhir/ValueSet/languages"
          }
        ],
        "text": "english"
      }
    }
  ],
  "generalPractitioner": [
    {
      "display": "test test test",
      "extension": [
        {
          "url": "http://test.com/exclude-test",
          "valueString": "test extension test test test"
        },
        {
          "url": "http://telus.com/fhir/created-at",
          "valueDateTime": "2025-10-09T10:39:14-07:00"
        }
      ],
      "id": "1",
      "reference": "Practitioner/1",
      "type": "Practitioner"
    }
  ],
  "identifier": [
    {
      "system": "urn:oid:2.16.840.1.113883.30.1467.6",
      "type": {
        "coding": [
          {
            "code": "MR",
            "system": "http://hl7.org/fhir/v2/0203"
          }
        ],
        "text": "CHR_ID"
      },
      "value": ".1"
    }
  ],
  "name": [
    {
      "family": "NEWWWWWW ",
      "given": [
        "YAYYYY"
      ],
      "use": "official"
    }
  ],
  "resourceType": "Patient",
  "telecom": [
    {
      "system": "phone",
      "use": "mobile",
      "value": "(888)369-3643"
    },
    {
      "system": "email",
      "use": "home",
      "value": "female.test@telus.com"
    }
  ]
}'