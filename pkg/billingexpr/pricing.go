package billingexpr

// FreePriceCandidates returns unit prices for free-model detection.
func FreePriceCandidates(exprStr, exprHash string, request RequestInput, promptTokens int, trace TraceResult) ([]float64, error) {
	if trace.BillingUnit == BillingUnitRequest && trace.FixedPrice != nil {
		return []float64{*trace.FixedPrice}, nil
	}

	usedVars := UsedVarsForGroup(exprStr, exprHash, request.Group)
	// image_count is a quantity multiplier. Free-model detection compares the
	// configured unit price, so do not let the requested batch size turn a
	// per-image price into a misleading total request price.
	candidateRequest := request
	if usedVars["image_count"] {
		unitImageCount := 1
		candidateRequest.ImageCount = &unitImageCount
	}
	baseParams := TokenParams{Len: float64(promptTokens)}
	baseCost, _, err := RunExprByHashWithRequest(exprStr, exprHash, baseParams, candidateRequest)
	if err != nil {
		return nil, err
	}
	prices := make([]float64, 0, 3)
	if usedVars["req"] {
		prices = append(prices, baseCost/1_000_000)
	}
	for variable, params := range map[string]TokenParams{
		"p": {P: 1, Len: float64(promptTokens)},
		"c": {C: 1, Len: float64(promptTokens)},
	} {
		if !usedVars[variable] {
			continue
		}
		cost, _, runErr := RunExprByHashWithRequest(exprStr, exprHash, params, candidateRequest)
		if runErr != nil {
			return nil, runErr
		}
		prices = append(prices, cost-baseCost)
	}
	return prices, nil
}
