/*
Copyright 2018 The KubeDB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/kubedb/apimachinery/apis/authorization/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeMongoDBRoleBindings implements MongoDBRoleBindingInterface
type FakeMongoDBRoleBindings struct {
	Fake *FakeAuthorizationV1alpha1
	ns   string
}

var mongodbrolebindingsResource = schema.GroupVersionResource{Group: "authorization.kubedb.com", Version: "v1alpha1", Resource: "mongodbrolebindings"}

var mongodbrolebindingsKind = schema.GroupVersionKind{Group: "authorization.kubedb.com", Version: "v1alpha1", Kind: "MongoDBRoleBinding"}

// Get takes name of the mongoDBRoleBinding, and returns the corresponding mongoDBRoleBinding object, and an error if there is any.
func (c *FakeMongoDBRoleBindings) Get(name string, options v1.GetOptions) (result *v1alpha1.MongoDBRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(mongodbrolebindingsResource, c.ns, name), &v1alpha1.MongoDBRoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MongoDBRoleBinding), err
}

// List takes label and field selectors, and returns the list of MongoDBRoleBindings that match those selectors.
func (c *FakeMongoDBRoleBindings) List(opts v1.ListOptions) (result *v1alpha1.MongoDBRoleBindingList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(mongodbrolebindingsResource, mongodbrolebindingsKind, c.ns, opts), &v1alpha1.MongoDBRoleBindingList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.MongoDBRoleBindingList{ListMeta: obj.(*v1alpha1.MongoDBRoleBindingList).ListMeta}
	for _, item := range obj.(*v1alpha1.MongoDBRoleBindingList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested mongoDBRoleBindings.
func (c *FakeMongoDBRoleBindings) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(mongodbrolebindingsResource, c.ns, opts))

}

// Create takes the representation of a mongoDBRoleBinding and creates it.  Returns the server's representation of the mongoDBRoleBinding, and an error, if there is any.
func (c *FakeMongoDBRoleBindings) Create(mongoDBRoleBinding *v1alpha1.MongoDBRoleBinding) (result *v1alpha1.MongoDBRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(mongodbrolebindingsResource, c.ns, mongoDBRoleBinding), &v1alpha1.MongoDBRoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MongoDBRoleBinding), err
}

// Update takes the representation of a mongoDBRoleBinding and updates it. Returns the server's representation of the mongoDBRoleBinding, and an error, if there is any.
func (c *FakeMongoDBRoleBindings) Update(mongoDBRoleBinding *v1alpha1.MongoDBRoleBinding) (result *v1alpha1.MongoDBRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(mongodbrolebindingsResource, c.ns, mongoDBRoleBinding), &v1alpha1.MongoDBRoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MongoDBRoleBinding), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeMongoDBRoleBindings) UpdateStatus(mongoDBRoleBinding *v1alpha1.MongoDBRoleBinding) (*v1alpha1.MongoDBRoleBinding, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(mongodbrolebindingsResource, "status", c.ns, mongoDBRoleBinding), &v1alpha1.MongoDBRoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MongoDBRoleBinding), err
}

// Delete takes name of the mongoDBRoleBinding and deletes it. Returns an error if one occurs.
func (c *FakeMongoDBRoleBindings) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(mongodbrolebindingsResource, c.ns, name), &v1alpha1.MongoDBRoleBinding{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeMongoDBRoleBindings) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(mongodbrolebindingsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.MongoDBRoleBindingList{})
	return err
}

// Patch applies the patch and returns the patched mongoDBRoleBinding.
func (c *FakeMongoDBRoleBindings) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.MongoDBRoleBinding, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(mongodbrolebindingsResource, c.ns, name, data, subresources...), &v1alpha1.MongoDBRoleBinding{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MongoDBRoleBinding), err
}
