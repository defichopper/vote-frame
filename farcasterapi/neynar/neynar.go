package neynar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/farcaster-poc/farcasterapi"
	"go.vocdoni.io/dvote/log"
	"go.vocdoni.io/dvote/util"
)

const (
	neynarHubEndpoint = "https://hub-api.neynar.com"
	neynarAPIEndpoint = "https://api.neynar.com"

	// endpoints
	neynarGetUsernameEndpoint = neynarAPIEndpoint + "/v1/farcaster/user?fid=%d"
	neynarGetCastsEndpoint    = neynarAPIEndpoint + "/v1/farcaster/mentions-and-replies?fid=%d&limit=150&cursor=%s"
	neynarReplyEndpoint       = neynarAPIEndpoint + "/v2/farcaster/cast"
	neynarUserByEthAddresses  = neynarAPIEndpoint + "/v2/farcaster/user/bulk-by-address?addresses=%s"
	neynarVerificationsByFID  = neynarHubEndpoint + "/v1/verificationsByFid?fid=%d"

	// timeouts
	getBotUsernameTimeout   = 10 * time.Second
	getCastByMentionTimeout = 60 * time.Second
	postCastTimeout         = 10 * time.Second

	// Requests backoff parameters
	maxConcurrentRequests = 2
	maxRetries            = 12              // Maximum number of retries
	baseDelay             = 1 * time.Second // Initial delay, increases exponentially

	// other
	neynarMentionType = "cast-mention"
	timeLayout        = "2006-01-02T15:04:05.000Z"
)

type NeynarAPI struct {
	fid          uint64
	username     string
	signerUUID   string
	apiKey       string
	reqSemaphore chan struct{} // Semaphore to limit concurrent requests

}

func NewNeynarAPI(apiKey string) *NeynarAPI {
	return &NeynarAPI{
		apiKey:       apiKey,
		reqSemaphore: make(chan struct{}, maxConcurrentRequests),
	}
}

// SetFarcasterUser method sets the farcaster user with the given fid and signer.
// The signer is the UUID of the user that signs the messages.
func (n *NeynarAPI) SetFarcasterUser(fid uint64, signer string) error {
	n.fid = fid
	n.signerUUID = signer
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), getBotUsernameTimeout)
	defer cancel()
	userdata, err := n.UserDataByFID(ctx, n.fid)
	if err != nil {
		return fmt.Errorf("error getting bot username: %w", err)
	}
	n.username = userdata.Username
	return nil
}

func (n *NeynarAPI) Stop() error {
	return nil
}

func (n *NeynarAPI) LastMentions(ctx context.Context, timestamp uint64) ([]*farcasterapi.APIMessage, uint64, error) {
	if n.fid == 0 {
		return nil, 0, fmt.Errorf("farcaster user not set")
	}

	messages := []*farcasterapi.APIMessage{}
	lastTimestamp := timestamp
	cursor := ""
	for {
		// create request with the given cursor and set the api key header
		url := fmt.Sprintf(neynarGetCastsEndpoint, n.fid, cursor)
		body, err := n.request(url, http.MethodGet, nil)
		if err != nil {
			return nil, 0, err
		}
		// decode mentions
		notificationsResponse := &NotificationsResponse{}
		if err := json.Unmarshal(body, notificationsResponse); err != nil {
			return nil, 0, fmt.Errorf("error unmarshalling response body: %w", err)
		}
		// parse mentions
		for _, notification := range notificationsResponse.Result.Notifications {
			// skip non-mentions
			if notification.Type != neynarMentionType {
				continue
			}
			// parse timestamp
			parsedTimestamp, err := time.Parse(timeLayout, notification.Timestamp)
			if err != nil {
				return nil, 0, fmt.Errorf("error parsing timestamp: %w", err)
			}
			// skip old mentions
			notificationTimestamp := uint64(parsedTimestamp.Unix())
			if notificationTimestamp <= timestamp {
				continue
			}
			// parse the text to remove the bot username and add mention to the
			// list
			mention := fmt.Sprintf("@%s", n.username)
			text := strings.TrimSpace(strings.TrimPrefix(notification.Text, mention))
			messages = append(messages, &farcasterapi.APIMessage{
				IsMention: true,
				Author:    notification.Author.FID,
				Content:   text,
				Hash:      notification.Hash,
			})
			// update last timestamp
			if notificationTimestamp > lastTimestamp {
				lastTimestamp = notificationTimestamp
			}
		}
		// stop if there are no new mentions
		if notificationsResponse.Result.NextCursor.Cursor == "" {
			break
		}
		cursor = notificationsResponse.Result.NextCursor.Cursor
	}
	return messages, lastTimestamp, nil
}

func (n *NeynarAPI) Reply(ctx context.Context, fid uint64, parentHash, content string) error {
	if n.fid == 0 {
		return fmt.Errorf("farcaster user not set")
	}
	// create request body
	castReq := &CastPostRequest{
		Signer: n.signerUUID,
		Text:   content,
		Parent: parentHash,
	}
	body, err := json.Marshal(castReq)
	if err != nil {
		return fmt.Errorf("error marshalling request body: %w", err)
	}
	// create request with the bot fid and set the api key header
	_, err = n.request(neynarReplyEndpoint, http.MethodPost, body)
	return err
}

// UserData method returns the username, the custody address and the
// verification addresses of the user with the given fid.
func (n *NeynarAPI) UserDataByFID(ctx context.Context, fid uint64) (*farcasterapi.Userdata, error) {
	// create request with the bot fid
	url := fmt.Sprintf(neynarGetUsernameEndpoint, fid)
	body, err := n.request(url, http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	// decode username
	usernameResponse := &UserdataResponse{}
	if err := json.Unmarshal(body, usernameResponse); err != nil {
		return nil, fmt.Errorf("error unmarshalling response body: %w", err)
	}

	// get signers
	signers, err := n.signersFromFid(fid)
	if err != nil {
		return nil, fmt.Errorf("error getting signers: %w", err)
	}

	return &farcasterapi.Userdata{
		FID:                    fid,
		Username:               usernameResponse.Result.User.Username,
		CustodyAddress:         usernameResponse.Result.User.CustodyAddress,
		VerificationsAddresses: usernameResponse.Result.User.VerificationsAddresses,
		Signers:                signers,
	}, nil
}

func (n *NeynarAPI) signersFromFid(fid uint64) ([]string, error) {
	// get verifications to fetch the signer
	body, err := n.request(fmt.Sprintf(neynarVerificationsByFID, fid), http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// decode verifications json
	verificationsData := &HubAPIResponse{}
	if err := json.Unmarshal(body, verificationsData); err != nil {
		return nil, fmt.Errorf("error unmarshalling verifications: %w", err)
	}
	// filter verifications addresses
	signersMap := make(map[string]struct{})
	for _, msg := range verificationsData.Messages {
		// if no data or verification data is found, skip. If the message data
		// type is not the one we are looking for, skip
		if msg.Data == nil || msg.Data.Type != HUB_MESSAGE_TYPE_VERIFICATION || msg.Data.VerificationAddEthAddressBody == nil || msg.Signer == "" {
			log.Warnw("invalid verification message", "msg", msg)
			continue
		}
		signersMap[msg.Signer] = struct{}{}
	}
	signers := []string{}
	for signer := range signersMap {
		signers = append(signers, strings.ToLower(signer))
	}
	return signers, nil
}

func (n *NeynarAPI) UserDataByVerificationAddress(ctx context.Context, address string) (*farcasterapi.Userdata, error) {
	// get the user data by the ethereum address
	ethAddress := common.HexToAddress(address)
	url := fmt.Sprintf(neynarUserByEthAddresses, ethAddress.Hex())
	body, err := n.request(url, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}

	// decode username
	results := make(map[string][]UserdataV2)
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("error unmarshalling response body: %w", err)
	}
	// take the first element of the results map which is the address, however
	// the address is not guaranteed to respect ethereum hex format.
	dataItems := []UserdataV2{}
	for _, v := range results {
		dataItems = v
		break
	}
	if len(dataItems) == 0 {
		return nil, farcasterapi.ErrNoDataFound
	}
	var data *UserdataV2
	for _, item := range dataItems {
		if item.Username != "" {
			data = &item
			break
		}
	}
	if data == nil {
		return nil, fmt.Errorf("no valid data found for the given address")
	}

	// get signers
	signers, err := n.signersFromFid(data.Fid)
	if err != nil {
		return nil, fmt.Errorf("error getting signers: %w", err)
	}
	// normalize addresses to the ethereum hex standard format
	var normalizedAddresses []string
	for _, addr := range data.VerifiedAddresses.EthAddresses {
		normalizedAddresses = append(normalizedAddresses, common.HexToAddress(addr).Hex())
	}
	return &farcasterapi.Userdata{
		FID:                    data.Fid,
		Username:               data.Username,
		CustodyAddress:         common.HexToAddress(data.CustodyAddress).Hex(),
		VerificationsAddresses: normalizedAddresses,
		Signers:                signers,
	}, nil
}

func (n *NeynarAPI) request(url, method string, body []byte) ([]byte, error) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("error creating request: %w", err)
		}
		req.Header.Set("api_key", n.apiKey)

		// We need to avoid too much concurrent requests and penalization from the API
		n.reqSemaphore <- struct{}{}
		res, err := http.DefaultClient.Do(req)
		<-n.reqSemaphore
		if err != nil {
			return nil, fmt.Errorf("error downloading json: %w", err)
		}
		defer res.Body.Close()
		if res.StatusCode == http.StatusTooManyRequests {
			time.Sleep(time.Duration(attempt+1)*baseDelay + time.Duration(util.RandomInt(0, 2000))*time.Millisecond)
		} else if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("error downloading json: %s", res.Status)
		} else {
			respBody, err := io.ReadAll(res.Body)
			if err != nil {
				return nil, fmt.Errorf("error reading response body: %w", err)
			}
			return respBody, nil // Success
		}
		log.Debugw("retrying request", "attempt", attempt+1, "url", url, "method", method)
	}

	return nil, fmt.Errorf("error downloading json: exceeded retry limit")
}
