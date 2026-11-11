// @recipe states-and-lgas States and LGAs
// @summary A Nigerian address uses the tax authority's own state and LGA codes. Read both lists, fill an LGA picker from the chosen state, save a buyer with the codes, and handle a value that is not on the lists.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"time"

	"github.com/elyonar/einvoice-go"
)

func main() {
	ctx := context.Background()

	// @step client Create the client
	yona, err := einvoice.New(os.Getenv("YONA_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	// @step lists Read the states and the LGAs
	// @text Both lists are free to read and change rarely, so read them once and keep them. CodeListItems decodes a list's items: a StateOption is {Code, Name}. An LGAOption is {Code, Name, StateCode}, where StateCode is its state's code.
	// @op getInvoiceReferenceList
	stateList, err := yona.Reference.GetResource(ctx, "states")
	if err != nil {
		log.Fatal(err)
	}
	states, err := einvoice.CodeListItems[einvoice.StateOption](stateList)
	if err != nil {
		log.Fatal(err)
	}
	lgaList, err := yona.Reference.GetResource(ctx, "lgas")
	if err != nil {
		log.Fatal(err)
	}
	lgas, err := einvoice.CodeListItems[einvoice.LGAOption](lgaList)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(states), "states,", len(lgas), "LGAs")

	// @step picker Fill the LGA picker from the chosen state
	// @text Filter the LGAs by the chosen state's code. Keep the code, never the name alone: a few LGA names exist in two states, and a code cannot be worked out from a name.
	at := slices.IndexFunc(states, func(state einvoice.StateOption) bool { return state.Name == "Lagos" })
	if at < 0 {
		log.Fatal("Lagos is not on the list of states")
	}
	lagos := states[at]
	var lgasOfLagos []einvoice.LGAOption
	for _, lga := range lgas {
		if lga.StateCode == lagos.Code {
			lgasOfLagos = append(lgasOfLagos, lga)
		}
	}
	at = slices.IndexFunc(lgasOfLagos, func(lga einvoice.LGAOption) bool { return lga.Name == "Ikeja" })
	if at < 0 {
		log.Fatal("Ikeja is not on the list of LGAs for Lagos")
	}
	ikeja := lgasOfLagos[at]
	fmt.Printf("%s %s (%d LGAs in Lagos)\n", lagos.Code, ikeja.Code, len(lgasOfLagos))

	// @step buyer Save a buyer with the codes
	// @text Send the state and LGA codes in the buyer's address. The exact names are accepted too and are stored as the codes; the answer always carries the codes. Both fields are optional.
	// @op createBuyer
	buyer, err := yona.Buyers.Create(ctx, &einvoice.CreateBuyerBody{
		Name:      fmt.Sprintf("Acme Trading %d", time.Now().UnixMilli()),
		PartyType: einvoice.Ptr(einvoice.CreateBuyerDtoPartyTypeIndividual),
		Address: &einvoice.BuyerAddressDto{
			Line1: "12 Allen Avenue", City: "Ikeja", State: einvoice.Ptr(lagos.Code), Lga: einvoice.Ptr(ikeja.Code), Country: "NG",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	saved := buyer.Address
	if saved == nil || saved.State == nil || *saved.State != lagos.Code || saved.Lga == nil || *saved.Lga != ikeja.Code {
		log.Fatal("the buyer did not keep the codes")
	}
	fmt.Println("buyer", buyer.ID, *saved.State, *saved.Lga)

	// @step refused Handle a value that is not on the lists
	// @text A state that is not on the list is refused with VAL202. An LGA that is not on the list, or that belongs to another state, is refused with VAL203. Nothing is saved, and Errors names the field.
	// @op updateBuyer
	at = slices.IndexFunc(lgas, func(lga einvoice.LGAOption) bool { return lga.StateCode != lagos.Code })
	if at < 0 {
		log.Fatal("every LGA is in Lagos")
	}
	elsewhere := lgas[at]
	_, err = yona.Buyers.Update(ctx, buyer.ID, &einvoice.UpdateBuyerBody{
		Address: &einvoice.BuyerAddressDto{
			Line1: "12 Allen Avenue", City: "Ikeja", State: einvoice.Ptr(lagos.Code), Lga: einvoice.Ptr(elsewhere.Code), Country: "NG",
		},
	})
	var refused *einvoice.ValidationError
	switch {
	case err == nil:
		log.Fatal("an LGA of another state was accepted")
	case !errors.As(err, &refused) || refused.ErrorCode != "VAL203":
		log.Fatal(err)
	}
	fmt.Println("refused:", refused.ErrorCode, refused.Errors)

	// @step tidy Remove the example buyer
	// @op deleteBuyer
	if _, err := yona.Buyers.Delete(ctx, buyer.ID); err != nil {
		log.Fatal(err)
	}
}
