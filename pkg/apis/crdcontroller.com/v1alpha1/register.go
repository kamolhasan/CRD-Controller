package v1alpha1

import (
	"github.com/kamolhasan/CRD-Controller/pkg/apis/crdcontroller.com"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group:crdcontroller_com.GroupName,Version:"v1alpha1"}

// kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource (resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// Adds the list of known types to Scheme
func addKnownTypes(scheme *runtime.Scheme)  error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Foo{},
		&FooList{},
		)
	metav1.AddToGroupVersion(scheme,SchemeGroupVersion)
	return nil

}


var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme	=	SchemeBuilder.AddToScheme
)