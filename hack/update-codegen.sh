#!/bin/sh
vendor/k8s.io/code-generator/generate-groups.sh all \
	github.com/kamolhasan/CRD-Controller/pkg/client \
	github.com/kamolhasan/CRD-Controller/pkg/apis \
	crdcontroller.com:v1alpha1