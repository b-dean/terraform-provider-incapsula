package incapsula

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"strconv"
)

// Endpoints (unexported consts)
const endpointSubAccountAdd = "subaccounts/add"
const endpointSubAccountList = "accounts/listSubAccounts"
const endpointSubAccountDelete = "subaccounts/delete"
const PAGE_SIZE = 50

type SubAccount struct {
	SubAccountID int `json:"sub_account_id"`
	*SubAccountPayload
}

// SubAccountAddResponse contains the relevant information when adding an Incapsula SubAccount
type SubAccountAddResponse struct {
	SubAccount SubAccount `json:"sub_account"`
	Res        int        `json:"res"`
}

// SubAccountListResponse contains list of Incapsula SubAccount
type SubAccountListResponse struct {
	SubAccounts []SubAccount `json:"resultList"`
	Res         int          `json:"res"`
}

// SubAccountPayload contains the payload for Incapsula SubAccount creation
type SubAccountPayload struct {
	SubAccountName string `json:"sub_account_name"`
	RefID          string `json:"ref_id,omitempty"`
	LogLevel       string `json:"log_level,omitempty"`
	ParentID       int    `json:"parent_id,omitempty"`
	LogsAccountID  int    `json:"logs_account_id",omitempty"`
}

// AddSubAccount adds a SubAccount to be managed by Incapsula
func (c *Client) AddSubAccount(subAccountPayload *SubAccountPayload) (*SubAccountAddResponse, error) {
	log.Printf("[INFO] Adding Incapsula subaccount: %s\n", subAccountPayload.SubAccountName)

	values := url.Values{
		"sub_account_name": {subAccountPayload.SubAccountName},
	}

	if subAccountPayload.RefID != "" {
		values["ref_id"] = make([]string, 1)
		values["ref_id"][0] = fmt.Sprint(subAccountPayload.RefID)
	}

	if subAccountPayload.ParentID != 0 {
		values["parent_id"] = make([]string, 1)
		values["parent_id"][0] = fmt.Sprint(subAccountPayload.ParentID)
	}

	if subAccountPayload.LogsAccountID != 0 {
		values["logs_account_id"] = make([]string, 1)
		values["logs_account_id"][0] = fmt.Sprint(subAccountPayload.LogsAccountID)
	}

	if subAccountPayload.LogLevel != "" {
		values["log_level"] = make([]string, 1)
		values["log_level"][0] = fmt.Sprint(subAccountPayload.LogLevel)
	}

	log.Printf("[DEBUG] parentID %d\n", subAccountPayload.ParentID)
	log.Printf("[DEBUG] logsAccountID %d\n", subAccountPayload.LogsAccountID)
	log.Printf("[DEBUG] logLevel %s\n", subAccountPayload.LogLevel)
	log.Printf("[DEBUG] refID %s\n", subAccountPayload.RefID)
	log.Printf("[DEBUG] values %s\n", values)

	resp, err := c.PostFormWithHeaders(fmt.Sprintf("%s/%s", c.config.BaseURL, endpointSubAccountAdd), values, CreateSubAccount)
	if err != nil {
		return nil, fmt.Errorf("Error adding subaccount %s: %s", subAccountPayload.SubAccountName, err)
	}

	// Read the body
	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)

	// Dump JSON
	log.Printf("[DEBUG] Incapsula add subaccount JSON response: %s\n", string(responseBody))

	// Parse the JSON
	var subAccountAddResponse SubAccountAddResponse
	err = json.Unmarshal([]byte(responseBody), &subAccountAddResponse)
	if err != nil {
		return nil, fmt.Errorf("Error parsing add subaccount JSON response for subaccount %s: %s", subAccountPayload.SubAccountName, err)
	}

	// Look at the response status code from Incapsula
	if subAccountAddResponse.Res != 0 {
		return nil, fmt.Errorf("Error from Incapsula service when adding subaccount %s: %s", subAccountPayload.SubAccountName, string(responseBody))
	}

	return &subAccountAddResponse, nil
}

// GetSubAccount gets the Incapsula list of SubAccounts
func (c *Client) GetSubAccount(parentAccountID int, subAccountID int) (*SubAccount, error) {

	log.Printf("[INFO] Reading Incapsula subaccounts for id: %d)", subAccountID)

	var count = 0
	var shouldFetch = true
	// Pagination (default page size 50)
	for shouldFetch {
		log.Printf("[DEBUG] looking for subaccount %d, fetching for page: %d", subAccountID, count)
		var subAccounts, error = c.sendListSubAccountsRequest(parentAccountID, count)
		if error != nil {
			return nil, error
		}
		for _, subAccount := range subAccounts {
			if subAccount.SubAccountID == subAccountID {
				log.Printf("[INFO] found subaccount : %v\n", subAccount)
				return &subAccount, nil
			}
		}
		shouldFetch = len(subAccounts) == PAGE_SIZE
		count += 1
	}
	log.Printf("[DEBUG] didn't find subaccount %d returning nil", subAccountID)
	return nil, nil
}

func (c *Client) sendListSubAccountsRequest(accountId int, pageNum int) ([]SubAccount, error) {
	values := map[string][]string{}

	if accountId != 0 {
		values["account_id"] = make([]string, 1)
		values["account_id"][0] = fmt.Sprint(accountId)
	}
	values["page_num"] = make([]string, 1)
	values["page_num"][0] = fmt.Sprint(pageNum)
	values["page_size"] = make([]string, 1)
	values["page_size"][0] = fmt.Sprint(PAGE_SIZE)

	log.Printf("[INFO] Pagination loop, page : %d)\n", pageNum)

	// Post form to Incapsula
	resp, err := c.PostFormWithHeaders(fmt.Sprintf("%s/%s", c.config.BaseURL, endpointSubAccountList), values, ReadSubAccount)
	if err != nil {
		return nil, fmt.Errorf("Error getting subaccounts for account %d: %s", accountId, err)
	}

	// Read the body
	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)

	// Dump JSON
	log.Printf("[DEBUG] Incapsula subaccounts JSON response: %s\n", string(responseBody))

	// Parse the JSON
	var subAccountListResponse SubAccountListResponse
	err = json.Unmarshal([]byte(responseBody), &subAccountListResponse)
	if err != nil {
		return nil, fmt.Errorf("Error parsing subaccounts list JSON response for accountid: %d %s\nresponse: %s", accountId, err, string(responseBody))
	}

	return subAccountListResponse.SubAccounts, nil
}

// DeleteSubAccount deletes a SubAcccount currently managed by Incapsula
func (c *Client) DeleteSubAccount(subAccountID int) error {
	// Specifically shaded this struct, no need to share across funcs or export
	// We only care about the response code and possibly the message
	type SubAccountDeleteResponse struct {
		Res        int    `json:"res"`
		ResMessage string `json:"res_message"`
	}

	log.Printf("[INFO] Deleting Incapsula subaccount id: %d\n", subAccountID)

	// Post form to Incapsula
	resp, err := c.PostFormWithHeaders(fmt.Sprintf("%s/%s", c.config.BaseURL, endpointSubAccountDelete), url.Values{
		"sub_account_id": {strconv.Itoa(subAccountID)},
	}, DeleteSubAccount)
	if err != nil {
		return fmt.Errorf("Error deleting subaccount id: %d: %s", subAccountID, err)
	}

	// Read the body
	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)

	// Dump JSON
	log.Printf("[DEBUG] Incapsula delete subaccount JSON response: %s\n", string(responseBody))

	// Parse the JSON
	var subaccountDeleteResponse SubAccountDeleteResponse
	err = json.Unmarshal([]byte(responseBody), &subaccountDeleteResponse)
	if err != nil {
		return fmt.Errorf("Error parsing delete account JSON response for subaccount id: %d: %s", subAccountID, err)
	}

	// Look at the response status code from Incapsula
	if subaccountDeleteResponse.Res != 0 {
		return fmt.Errorf("Error from Incapsula service when deleting subaccount id: %d: %s", subAccountID, string(responseBody))
	}

	return nil
}
