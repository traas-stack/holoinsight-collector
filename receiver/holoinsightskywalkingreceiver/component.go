// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package holoinsightskywalkingreceiver

import (
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"strconv"
)

var (
	componentName2Id     = make(map[string]int32, 200)
	componentID2Name     = make(map[int32]string, 200)
	componentID2ServerID = make(map[int32]int32, 60)
)

const ComponentServerMappingSection = "Component-Server-Mappings"
const NoneComponent = "N/A"

func init() {
	buf, err := os.ReadFile("./config/component-libraries.yml")
	if err != nil {
		log.Printf("load component-libraries.yml error: %v", err)
		return
	}

	data := make(map[string]map[string]string)
	err = yaml.Unmarshal(buf, data)
	if err != nil {
		log.Printf("component-libraries.yml unmarshal error: %v", err)
	}

	nameMapping := make(map[string]string)

	for componentName, value := range data {
		if ComponentServerMappingSection == componentName {
			for name, serverName := range value {
				nameMapping[name] = serverName
			}
		} else {
			componentID := value["id"]
			componentIDInt, _ := strconv.ParseInt(componentID, 10, 32)
			componentName2Id[componentName] = int32(componentIDInt)
			componentID2Name[int32(componentIDInt)] = componentName
		}
	}

	for name, serverName := range nameMapping {
		if _, ok := componentName2Id[name]; !ok {
			log.Printf("Component name [" + name + "] in Component-Server-Mappings doesn't exist in component define")
		}
		if _, ok := componentName2Id[serverName]; !ok {
			log.Printf("Component name [" + serverName + "] in Component-Server-Mappings doesn't exist in component define")
		}

		componentID2ServerID[componentName2Id[name]] = componentName2Id[serverName]
	}

}

func getComponentName(componentID int32) string {
	componentName, ok := componentID2Name[componentID]
	if !ok {
		componentName = NoneComponent
	}

	return componentName
}

func getServerNameBasedOnComponent(componentID int32) string {
	serverComponentID, ok := componentID2ServerID[componentID]
	if !ok {
		return getComponentName(componentID)
	}

	return getComponentName(serverComponentID)
}
