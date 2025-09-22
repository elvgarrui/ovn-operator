/*
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

package ovncontroller

import (
	"context"
	"fmt"
	"unicode"

	netattdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
)

// CreateOrUpdateAdditionalNetworks - create or update network attachment definitions based on the provided mappings
func CreateOrUpdateAdditionalNetworks(
	ctx context.Context,
	h *helper.Helper,
	instance *ovnv1.OVNController,
	labels map[string]string,
) ([]string, error) {
	var networkAttachments []string

	for physNet, interfaceName := range instance.Spec.NicMappings {
		nadConfig := defineNADSpecConfig(physNet, interfaceName)
		err := createOrUpdateNetworkAttachmentDefinition(ctx, h, instance, labels, physNet, nadConfig)
		if err != nil {
			return nil, err
		}
		networkAttachments = append(networkAttachments, physNet)
	}

	return networkAttachments, nil
}

// CreateOrUpdateBonsNADs - create NADs for bond members and for bond
func CreateOrUpdateBondNADs(
	ctx context.Context,
	h *helper.Helper,
	instance *ovnv1.OVNController,
) ([]string, error) {
	var networkAttachments []string

	for bondName, bond := range instance.Spec.BondConfiguration {
		var linkNames []string
		for _, interfaceName := range bond.Links {

			var memberRunes []rune
			for _, char := range interfaceName {
				// Keep alphanumeric characters and the hyphen
				if unicode.IsLetter(char) || unicode.IsNumber(char) || char == '-' {
					memberRunes = append(memberRunes, char)
				} else {
					memberRunes = append(memberRunes, '-')
				}
			}
			memberName := string(memberRunes)
			nadConfig := defineNADSpecConfig(memberName, interfaceName)
			err := createOrUpdateNetworkAttachmentDefinition(ctx, h, instance, nil, memberName, nadConfig)
			if err != nil {
				return nil, err
			}
			networkAttachments = append(networkAttachments, memberName)
			linkNames = append(linkNames, memberName)
		}
		mainBondNadConfig := defineMainBondNADSpecConfig(bondName, bond.Mode, linkNames)
		err := createOrUpdateNetworkAttachmentDefinition(ctx, h, instance, nil, bondName, mainBondNadConfig)
		if err != nil {
			return nil, err
		}
		networkAttachments = append(networkAttachments, bondName)
	}
	return networkAttachments, nil
}

func defineNADSpecConfig(
	physNet string,
	interfaceName string,
) string {
	return fmt.Sprintf(
		`{"cniVersion": "0.3.1", "name": "%s", "type": "host-device", "device": "%s"}`,
		physNet, interfaceName)
}

func defineMainBondNADSpecConfig(
	physNet string,
	mode string,
	links []string,
) string {
	linkNames := `[`
	for i, link := range links {
		linkNames = linkNames + fmt.Sprintf(`{"name": "%s"}`, link)
		if i < len(links)-1 {
			linkNames = linkNames + ", "
		}
	}
	linkNames = linkNames + `]`

	return fmt.Sprintf(
		`{"cniVersion": "0.3.1","name": "%s", "type": "bond", "mode": "%s", "failOverMac": 1, "linksInContainer": true, "miimon": "100", "mtu": 1500, "links": %s}`,
		physNet, mode, linkNames)
}

func createOrUpdateNetworkAttachmentDefinition(
	ctx context.Context,
	h *helper.Helper,
	instance *ovnv1.OVNController,
	labels map[string]string,
	physNet string,
	nadSpecConfig string,
) error {
	var nad *netattdefv1.NetworkAttachmentDefinition

	nadSpec := netattdefv1.NetworkAttachmentDefinitionSpec{
		Config: nadSpecConfig,
	}
	nad = &netattdefv1.NetworkAttachmentDefinition{}
	err := h.GetClient().Get(
		ctx,
		client.ObjectKey{
			Namespace: instance.Namespace,
			Name:      physNet,
		},
		nad,
	)
	if err != nil {
		if !k8s_errors.IsNotFound(err) {
			return fmt.Errorf("cannot get NetworkAttachmentDefinition %s: %w", nadSpecConfig, err)
		}

		ownerRef := metav1.NewControllerRef(instance, instance.GroupVersionKind())
		nad = &netattdefv1.NetworkAttachmentDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:            physNet,
				Namespace:       instance.Namespace,
				Labels:          labels,
				OwnerReferences: []metav1.OwnerReference{*ownerRef},
			},
			Spec: nadSpec,
		}
		// Request object not found, lets create it
		if err := h.GetClient().Create(ctx, nad); err != nil {
			return fmt.Errorf("cannot create NetworkAttachmentDefinition %s: %w", nadSpecConfig, err)
		}
	} else {
		owned := false
		for _, owner := range nad.GetOwnerReferences() {
			if owner.Name == instance.Name {
				owned = true
				break
			}
		}
		if owned {
			nad.Spec = nadSpec
			if err := h.GetClient().Update(ctx, nad); err != nil {
				return fmt.Errorf("cannot update NetworkAttachmentDefinition %s: %w", nadSpecConfig, err)
			}
		}
	}

	return nil
}
